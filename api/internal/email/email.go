// Package email sends transactional email messages for the ZaaS auth flow
// using SMTP with STARTTLS.
package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	ttemplate "text/template"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

//go:embed templates/*.html templates/*.txt
var templateFS embed.FS

// Sender sends transactional emails.
type Sender interface {
	SendVerification(ctx context.Context, to, verificationURL string) error
	SendReissue(ctx context.Context, to, verificationURL string) error
}

// SMTPConfig holds SMTP connection parameters.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type smtpSender struct {
	cfg       SMTPConfig
	htmlTmpls *template.Template
	textTmpls *ttemplate.Template
}

// NewSMTPSender creates a Sender that delivers via SMTP.
func NewSMTPSender(cfg SMTPConfig) Sender {
	htmlTmpls := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	textTmpls := ttemplate.Must(ttemplate.ParseFS(templateFS, "templates/*.txt"))
	return &smtpSender{cfg: cfg, htmlTmpls: htmlTmpls, textTmpls: textTmpls}
}

type templateData struct {
	VerificationURL string
}

func (s *smtpSender) SendVerification(ctx context.Context, to, verificationURL string) error {
	data := templateData{VerificationURL: verificationURL}
	return s.send(ctx, to, "Verify your email", "verification", data)
}

func (s *smtpSender) SendReissue(ctx context.Context, to, verificationURL string) error {
	data := templateData{VerificationURL: verificationURL}
	return s.send(ctx, to, "Re-issue your API key", "reissue", data)
}

// sanitizeHeader strips CR and LF characters from a header value as a
// defense-in-depth measure against SMTP header injection.
func sanitizeHeader(v string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(v)
}

func (s *smtpSender) send(ctx context.Context, to, subject, templateName string, data templateData) error {
	tracer := otel.Tracer("zaas/api/internal/email")
	_, span := tracer.Start(ctx, "email.send")
	span.SetAttributes(
		attribute.String("email.template", templateName),
		attribute.String("smtp.host", s.cfg.Host),
	)
	defer span.End()

	msgBytes, err := s.buildMessage(to, subject, templateName, data)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	from := s.cfg.From
	if idx := strings.Index(from, "<"); idx >= 0 {
		from = strings.TrimRight(from[idx+1:], ">")
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}
	if err := smtp.SendMail(addr, auth, from, []string{to}, msgBytes); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// buildMessage renders HTML and text templates and assembles the MIME multipart message.
func (s *smtpSender) buildMessage(to, subject, templateName string, data templateData) ([]byte, error) {
	var htmlBuf bytes.Buffer
	if err := s.htmlTmpls.ExecuteTemplate(&htmlBuf, templateName+".html", data); err != nil {
		return nil, fmt.Errorf("render HTML template %s: %w", templateName, err)
	}

	var textBuf bytes.Buffer
	if err := s.textTmpls.ExecuteTemplate(&textBuf, templateName+".txt", data); err != nil {
		return nil, fmt.Errorf("render text template %s: %w", templateName, err)
	}

	boundary := "zaas-boundary-000"
	var msg bytes.Buffer
	msg.WriteString("From: " + sanitizeHeader(s.cfg.From) + "\r\n")
	msg.WriteString("To: " + sanitizeHeader(to) + "\r\n")
	msg.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	msg.WriteString("\r\n")

	// Plain text part
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	qpText := quotedprintable.NewWriter(&msg)
	_, _ = qpText.Write(textBuf.Bytes())
	_ = qpText.Close()
	msg.WriteString("\r\n")

	// HTML part
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	msg.WriteString("\r\n")
	qpHTML := quotedprintable.NewWriter(&msg)
	_, _ = qpHTML.Write(htmlBuf.Bytes())
	_ = qpHTML.Close()
	msg.WriteString("\r\n")

	msg.WriteString("--" + boundary + "--\r\n")
	return msg.Bytes(), nil
}
