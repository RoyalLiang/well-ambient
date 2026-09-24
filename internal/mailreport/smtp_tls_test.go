package mailreport

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func smtpTestCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}
func TestSMTPHandshakeModesAndAuthentication(t *testing.T) {
	cert, roots := smtpTestCertificate(t)
	for _, mode := range []string{"tls", "starttls"} {
		t.Run(mode, func(t *testing.T) {
			accepted := make(chan bool, 1)
			cfg, done := localSMTP(t, func(raw net.Conn) {
				conn := raw
				secured := false
				secure := func() bool {
					tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
					if err := tlsConn.Handshake(); err != nil {
						return false
					}
					conn = tlsConn
					secured = true
					return true
				}
				if mode == "tls" && !secure() {
					return
				}
				reply := func(value string) { _, _ = conn.Write([]byte(value + "\r\n")) }
				reply("220 local TLS SMTP")
				reader := textproto.NewReader(bufio.NewReader(conn))
				authenticated := false
				for {
					line, err := reader.ReadLine()
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						if secured {
							reply("250-localhost\r\n250 AUTH PLAIN")
						} else {
							reply("250-localhost\r\n250 STARTTLS")
						}
					case line == "STARTTLS":
						reply("220 upgrade")
						if !secure() {
							return
						}
						reader = textproto.NewReader(bufio.NewReader(conn))
					case strings.HasPrefix(line, "AUTH PLAIN "):
						if !secured {
							t.Error("credentials sent before TLS")
							return
						}
						authenticated = true
						reply("235 authenticated")
					case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
						reply("250 ok")
					case line == "DATA":
						reply("354 data")
						if _, err := reader.ReadDotBytes(); err != nil {
							return
						}
						accepted <- authenticated
						reply("250 accepted")
					case line == "QUIT":
						reply("221 bye")
						return
					default:
						reply("500 unsupported")
					}
				}
			})
			cfg.TLSMode = mode
			cfg.Username = "fixture"
			cfg.Password = "local-test-secret"
			if err := sendWithRoots(context.Background(), cfg, []string{"recipient@example.test"}, "TLS test", "text", "<p>text</p>", roots); err != nil {
				t.Fatal(err)
			}
			<-done
			if !<-accepted {
				t.Fatal("TLS SMTP skipped authentication")
			}
		})
	}
}
func TestSMTPRejectsUntrustedCertificate(t *testing.T) {
	cert, _ := smtpTestCertificate(t)
	cfg, _ := localSMTP(t, func(raw net.Conn) {
		conn := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		_ = conn.Handshake()
	})
	cfg.TLSMode = "tls"
	if err := Send(context.Background(), cfg, []string{"recipient@example.test"}, "TLS test", "text", "html"); err == nil || !strings.Contains(err.Error(), "TLS handshake") {
		t.Fatalf("untrusted certificate accepted: %v", err)
	}
}
