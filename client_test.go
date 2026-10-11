package app_store

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"testing"
)

func TestNewClient(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("kid", "issuer", "bundle", key)

	client, err := NewClient(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("client should not be nil")
	}
	if client.baseURL != ProductionBaseURL {
		t.Errorf("expected baseURL %s, got %s", ProductionBaseURL, client.baseURL)
	}
}

func TestNewClientNilToken(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected error for nil token")
	}
}

func TestNewClientWithSandbox(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("kid", "issuer", "bundle", key)

	client, err := NewClient(token, WithSandbox())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.baseURL != SandboxBaseURL {
		t.Errorf("expected baseURL %s, got %s", SandboxBaseURL, client.baseURL)
	}
}

func TestNewClientWithHttpClient(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("kid", "issuer", "bundle", key)
	customClient := &http.Client{}

	client, err := NewClient(token, WithHttpClient(customClient))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.client != customClient {
		t.Error("custom http client not set")
	}
}
