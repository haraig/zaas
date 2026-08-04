package email_test

import (
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"strconv"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"zaas/api/internal/email"
)

// mockSMTPServer starts a minimal SMTP server that captures the message.
// Returns the listener and a channel that receives the raw message data.
func mockSMTPServer(t *testing.T) (net.Listener, <-chan string) {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	msgCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// SMTP conversation
		write := func(s string) { conn.Write([]byte(s + "\r\n")) } //nolint:errcheck
		read := func() {
			buf := make([]byte, 4096)
			conn.Read(buf) //nolint:errcheck
		}

		write("220 localhost SMTP")
		read() // EHLO
		write("250-localhost\r\n250 AUTH PLAIN LOGIN")
		read() // AUTH
		write("235 Authentication successful")
		read() // MAIL FROM
		write("250 OK")
		read() // RCPT TO
		write("250 OK")
		read() // DATA
		write("354 Start mail input")

		// Read until ".\r\n"
		var body strings.Builder
		for {
			buf := make([]byte, 4096)
			n, err := conn.Read(buf)
			if err != nil {
				break
			}
			body.Write(buf[:n])
			if strings.Contains(body.String(), "\r\n.\r\n") {
				break
			}
		}
		write("250 OK")
		read() // QUIT
		write("221 Bye")

		msgCh <- body.String()
	}()
	return ln, msgCh
}

// decodeQP decodes all quoted-printable sequences in a string.
// It splits on boundaries and decodes each QP section independently.
func decodeQP(s string) string {
	r := quotedprintable.NewReader(strings.NewReader(s))
	decoded, err := io.ReadAll(r)
	if err != nil {
		// Fall back to raw if decoding fails
		return s
	}
	return string(decoded)
}

func TestSendVerification(t *testing.T) {
	t.Parallel()
	ln, msgCh := mockSMTPServer(t)
	defer ln.Close()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	sender := email.NewSMTPSender(email.SMTPConfig{
		Host:     host,
		Port:     port,
		User:     "test@zaas.at",
		Password: "secret",
		From:     "ZaaS <noreply@zaas.at>",
	})

	err := sender.SendVerification(context.Background(), "user@example.com", "https://zaas.at/api/v1/auth/verify?token=abc123&type=registration")
	if err != nil {
		t.Fatalf("SendVerification: %v", err)
	}

	msg := <-msgCh
	decoded := decodeQP(msg)
	if !strings.Contains(decoded, "Verify your email") {
		t.Error("message should contain subject 'Verify your email'")
	}
	if !strings.Contains(decoded, "https://zaas.at/api/v1/auth/verify?token=abc123&type=registration") {
		t.Error("message should contain the verification URL")
	}
	if !strings.Contains(decoded, "multipart/alternative") {
		t.Error("message should be multipart (HTML + text)")
	}
}

func TestSendVerification_CRLFToAddress_NoExtraHeaders(t *testing.T) {
	t.Parallel()
	ln, msgCh := mockSMTPServer(t)
	defer ln.Close()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	sender := email.NewSMTPSender(email.SMTPConfig{
		Host:     host,
		Port:     port,
		User:     "test@zaas.at",
		Password: "secret",
		From:     "ZaaS <noreply@zaas.at>",
	})

	// Attempt to inject a Bcc header via a CRLF-laden To address.
	// Defense-in-depth: sanitizeHeader strips CR/LF from the To: header line,
	// and the Go smtp package also rejects CRLF in recipient addresses.
	// Either behavior (error returned OR no injected header in the message) is acceptable.
	maliciousTo := "victim@x.com\r\nBcc: attacker@evil.com"
	err := sender.SendVerification(context.Background(), maliciousTo, "https://zaas.at/verify")
	if err != nil {
		// smtp rejected the CRLF - defense worked at the smtp layer.
		return
	}

	// If smtp accepted it (sanitizeHeader stripped the CRLF before smtp saw it),
	// verify no injected headers appear in the outgoing message.
	msg := <-msgCh
	if strings.Contains(msg, "Bcc:") {
		t.Error("outgoing message must not contain injected Bcc header")
	}
	if strings.Contains(msg, "attacker@evil.com") {
		t.Error("outgoing message must not contain attacker address")
	}
}

func TestSendVerification_CreatesOTelSpan(t *testing.T) {
	ln, _ := mockSMTPServer(t)
	defer ln.Close()

	// Set up an in-memory span recorder.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	sender := email.NewSMTPSender(email.SMTPConfig{
		Host:     host,
		Port:     port,
		User:     "test@zaas.at",
		Password: "secret",
		From:     "ZaaS <noreply@zaas.at>",
	})

	ctx := context.Background()
	_ = sender.SendVerification(ctx, "user@example.com", "https://zaas.at/verify?token=x&type=registration")

	spans := exporter.GetSpans()
	var found bool
	for _, s := range spans {
		if s.Name == "email.send" {
			found = true
			// Verify the template attribute is present.
			var hasTemplate bool
			for _, attr := range s.Attributes {
				if string(attr.Key) == "email.template" {
					hasTemplate = true
				}
			}
			if !hasTemplate {
				t.Error("span should carry email.template attribute")
			}
		}
	}
	if !found {
		t.Errorf("expected span named 'email.send', got %d span(s)", len(spans))
	}
}

func TestSendReissue_CreatesOTelSpan(t *testing.T) {
	ln, _ := mockSMTPServer(t)
	defer ln.Close()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	sender := email.NewSMTPSender(email.SMTPConfig{
		Host:     host,
		Port:     port,
		User:     "test@zaas.at",
		Password: "secret",
		From:     "ZaaS <noreply@zaas.at>",
	})

	ctx := context.Background()
	_ = sender.SendReissue(ctx, "user@example.com", "https://zaas.at/verify?token=y&type=reissue")

	spans := exporter.GetSpans()
	var found bool
	for _, s := range spans {
		if s.Name == "email.send" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected span named 'email.send', got %d span(s)", len(spans))
	}
}

func TestSendReissue(t *testing.T) {
	t.Parallel()
	ln, msgCh := mockSMTPServer(t)
	defer ln.Close()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	sender := email.NewSMTPSender(email.SMTPConfig{
		Host:     host,
		Port:     port,
		User:     "test@zaas.at",
		Password: "secret",
		From:     "ZaaS <noreply@zaas.at>",
	})

	err := sender.SendReissue(context.Background(), "user@example.com", "https://zaas.at/api/v1/auth/verify?token=def456&type=reissue")
	if err != nil {
		t.Fatalf("SendReissue: %v", err)
	}

	msg := <-msgCh
	decoded := decodeQP(msg)
	if !strings.Contains(decoded, "Re-issue your API key") {
		t.Error("message should contain subject 'Re-issue your API key'")
	}
	if !strings.Contains(decoded, "https://zaas.at/api/v1/auth/verify?token=def456&type=reissue") {
		t.Error("message should contain the verification URL")
	}
}
