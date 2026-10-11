package app_store

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestDecodeCertificate(t *testing.T) {
	// Test with Apple root certificate
	cert, err := DecodeCertificate([]byte(AppleRootCAG3))
	if err != nil {
		t.Fatalf("failed to decode certificate: %v", err)
	}
	if cert == nil {
		t.Fatal("certificate should not be nil")
	}
	if cert.Issuer.Organization[0] != "Apple Inc." {
		t.Errorf("expected Apple Inc., got %s", cert.Issuer.Organization[0])
	}
}

func TestDecodeCertificateEmpty(t *testing.T) {
	_, err := DecodeCertificate([]byte{})
	if err == nil {
		t.Fatal("expected error for empty certificate")
	}
}

func TestDecodeCertificateInvalid(t *testing.T) {
	_, err := DecodeCertificate([]byte("not a certificate"))
	if err == nil {
		t.Fatal("expected error for invalid certificate")
	}
}

func TestX5CRootsAddAndMatch(t *testing.T) {
	verifier := &X5CRoots{}
	err := verifier.AddRoot(AppleRootCAG3)
	if err != nil {
		t.Fatalf("failed to add root: %v", err)
	}

	cert, err := DecodeCertificate([]byte(AppleRootCAG3))
	if err != nil {
		t.Fatalf("failed to decode certificate: %v", err)
	}

	matched, ok := verifier.MatchRoot(cert)
	if !ok {
		t.Fatal("expected root to be found")
	}
	if matched == nil {
		t.Fatal("matched cert should not be nil")
	}
}

func TestX5CRootsNoMatch(t *testing.T) {
	verifier := &X5CRoots{}
	err := verifier.AddRoot(AppleRootCAG3)
	if err != nil {
		t.Fatalf("failed to add root: %v", err)
	}

	cert, err := DecodeCertificate([]byte(AppleIncRootCertificate))
	if err != nil {
		t.Fatalf("failed to decode certificate: %v", err)
	}

	_, ok := verifier.MatchRoot(cert)
	if ok {
		t.Fatal("should not match a different certificate")
	}
}

func TestX5CVerifierInit(t *testing.T) {
	// Verify the global verifier has all 4 roots
	certs := []string{AppleIncRootCertificate, AppleComputerRootCertificate, AppleRootCAG2, AppleRootCAG3}
	for _, c := range certs {
		cert, err := DecodeCertificate([]byte(c))
		if err != nil {
			t.Fatalf("failed to decode certificate: %v", err)
		}
		_, ok := X5CVerifier.MatchRoot(cert)
		if !ok {
			t.Errorf("global verifier should contain all Apple root certificates")
		}
	}
}

func TestParsePrivateKey(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})

	parsed, err := ParsePrivateKey(string(pemBlock))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}
	if parsed == nil {
		t.Fatal("parsed key should not be nil")
	}
}

func TestParsePrivateKeyWithoutPEMHeader(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(key)

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})

	// Strip PEM headers
	pemStr := string(pemBlock)
	pemStr = pemStr[len("-----BEGIN PRIVATE KEY-----\n"):]
	pemStr = pemStr[:len(pemStr)-len("-----END PRIVATE KEY-----\n")]

	parsed, err := ParsePrivateKey(pemStr)
	if err != nil {
		t.Fatalf("failed to parse private key without header: %v", err)
	}
	if parsed == nil {
		t.Fatal("parsed key should not be nil")
	}
}

func TestDecodeCertificateWithoutPEMHeader(t *testing.T) {
	// Extract raw base64 from a PEM certificate
	block, _ := pem.Decode([]byte(AppleRootCAG3))
	if block == nil {
		t.Fatal("failed to decode PEM")
	}

	// Re-encode as raw base64 (without PEM headers)
	rawB64 := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: block.Bytes,
	})
	// Strip headers for raw test
	raw := string(rawB64)
	raw = raw[len("-----BEGIN CERTIFICATE-----\n"):]
	raw = raw[:len(raw)-len("-----END CERTIFICATE-----\n")]

	cert, err := DecodeCertificate([]byte(raw))
	if err != nil {
		t.Fatalf("failed to decode raw certificate: %v", err)
	}
	if cert == nil {
		t.Fatal("cert should not be nil")
	}
}
