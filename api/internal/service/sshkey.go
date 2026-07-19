package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	gossh "golang.org/x/crypto/ssh"
)

// ErrInvalidKeyType is returned when an unsupported key type is given.
var ErrInvalidKeyType = errors.New("invalid key type")

// ErrInvalidKeyBits is returned when an unsupported RSA bit size is given.
var ErrInvalidKeyBits = errors.New("invalid key bits")

// SSHKeyPair holds a generated SSH keypair.
type SSHKeyPair struct {
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

// GenerateSSHKeys generates `count` SSH keypairs.
// keyType: "ed25519" or "rsa". bits: 2048 or 4096 (RSA only).
// comment: appended to the public key (may be empty).
// count must be 1-5.
func GenerateSSHKeys(count int, keyType string, bits int, comment string) ([]SSHKeyPair, error) {
	if keyType != "ed25519" && keyType != "rsa" {
		return nil, fmt.Errorf("%w: key type must be ed25519 or rsa; got %q", ErrInvalidKeyType, keyType)
	}
	if keyType == "rsa" && bits != 2048 && bits != 4096 {
		return nil, fmt.Errorf("%w: RSA bits must be 2048 or 4096; got %d", ErrInvalidKeyBits, bits)
	}
	if len(comment) > 128 {
		return nil, fmt.Errorf("%w: comment must be at most 128 characters; got %d", ErrInvalidParam, len(comment))
	}
	if count < 1 || count > 5 {
		return nil, fmt.Errorf("%w: count must be between 1 and 5; got %d", ErrCountOutOfRange, count)
	}

	results := make([]SSHKeyPair, count)
	for i := range results {
		kp, err := generateSSHKeyPair(keyType, bits, comment)
		if err != nil {
			return nil, fmt.Errorf("generating keypair %d: %w", i+1, err)
		}
		results[i] = kp
	}
	return results, nil
}

func generateSSHKeyPair(keyType string, bits int, comment string) (SSHKeyPair, error) {
	switch keyType {
	case "ed25519":
		return generateEd25519(comment)
	case "rsa":
		return generateRSA(bits, comment)
	default:
		return SSHKeyPair{}, ErrInvalidKeyType
	}
}

func generateEd25519(comment string) (SSHKeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return SSHKeyPair{}, err
	}

	privPEM, err := marshalEd25519PrivateKey(priv)
	if err != nil {
		return SSHKeyPair{}, err
	}

	pubKey, err := gossh.NewPublicKey(pub)
	if err != nil {
		return SSHKeyPair{}, err
	}

	return SSHKeyPair{
		PrivateKey:  string(privPEM),
		PublicKey:   publicKeyString(pubKey, comment),
		Fingerprint: gossh.FingerprintSHA256(pubKey),
	}, nil
}

func generateRSA(bits int, comment string) (SSHKeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return SSHKeyPair{}, err
	}

	privDER := x509.MarshalPKCS1PrivateKey(priv)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	})

	pubKey, err := gossh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return SSHKeyPair{}, err
	}

	return SSHKeyPair{
		PrivateKey:  string(privPEM),
		PublicKey:   publicKeyString(pubKey, comment),
		Fingerprint: gossh.FingerprintSHA256(pubKey),
	}, nil
}

// marshalEd25519PrivateKey encodes an ed25519 private key as OpenSSH PEM.
func marshalEd25519PrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	// MarshalPrivateKey produces OpenSSH format which is what ssh-keygen outputs.
	pemBlock, err := gossh.MarshalPrivateKey(key, "")
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(pemBlock), nil
}

func publicKeyString(pub gossh.PublicKey, comment string) string {
	b := gossh.MarshalAuthorizedKey(pub)
	line := string(b)
	// MarshalAuthorizedKey ends with \n; strip it to add comment
	line = line[:len(line)-1]
	if comment != "" {
		line += " " + comment
	}
	return line + "\n"
}
