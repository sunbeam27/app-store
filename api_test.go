package app_store

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	token := NewToken("kid", "issuer", "bundle", key)
	// Pre-generate token

	client, err := NewClient(token, WithHttpClient(server.Client()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.baseURL = server.URL
	return client
}

func TestLookupOrder_Success(t *testing.T) {
	status := int32(0)
	expected := &OrderLookupResponse{
		Status:             &status,
		SignedTransactions: []string{},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}

	client := newTestClient(t, handler)
	resp, err := client.LookupOrder(context.Background(), "test-order-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if *resp.Status != 0 {
		t.Errorf("expected status 0, got %d", *resp.Status)
	}
}

func TestLookupOrder_Unauthorized(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}

	client := newTestClient(t, handler)
	_, err := client.LookupOrder(context.Background(), "test-order-id")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestLookupOrder_ErrorResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			ErrorCode:    4000001,
			ErrorMessage: "Invalid request",
		})
	}

	client := newTestClient(t, handler)
	_, err := client.LookupOrder(context.Background(), "test-order-id")
	if err == nil {
		t.Fatal("expected error")
	}
	e := UnwrapError(err)
	if e == nil {
		t.Fatal("expected ErrorResponse")
	}
	if e.ErrorCode != 4000001 {
		t.Errorf("expected error code 4000001, got %d", e.ErrorCode)
	}
}

func TestGetTransactionInfo_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		// Return a mock response - will fail on JWT decode but tests the HTTP layer
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"signedTransactionInfo": "invalid.jwt.token",
		})
	}

	client := newTestClient(t, handler)
	_, err := client.GetTransactionInfo(context.Background(), "test-tx-id")
	// This will error because we can't create a valid signed JWT for testing
	if err == nil {
		t.Fatal("expected error due to invalid JWT")
	}
}

func TestGetTransactionInfo_Unauthorized(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}

	client := newTestClient(t, handler)
	_, err := client.GetTransactionInfo(context.Background(), "test-tx-id")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestGetTransactionHistory_Success(t *testing.T) {
	expected := TransactionHistoryResponse{
		BundleId:           "com.test.app",
		HasMore:            false,
		Revision:           "rev1",
		SignedTransactions: []string{},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}

	client := newTestClient(t, handler)
	resp, err := client.GetTransactionHistory(context.Background(), "test-tx-id", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.BundleId != "com.test.app" {
		t.Errorf("expected bundleId 'com.test.app', got '%s'", resp.BundleId)
	}
	if resp.HasMore {
		t.Error("expected hasMore to be false")
	}
}

func TestGetTransactionHistory_WithRevision(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		revision := r.URL.Query().Get("revision")
		if revision != "rev123" {
			t.Errorf("expected revision 'rev123', got '%s'", revision)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TransactionHistoryResponse{})
	}

	client := newTestClient(t, handler)
	_, err := client.GetTransactionHistory(context.Background(), "test-tx-id", "rev123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetAllSubscriptionStatuses_Success(t *testing.T) {
	expected := StatusResponse{
		BundleId:    "com.test.app",
		Environment: "Sandbox",
		Data:        []SubscriptionGroupIdentity{},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}

	client := newTestClient(t, handler)
	resp, err := client.GetAllSubscriptionStatuses(context.Background(), "test-tx-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.BundleId != "com.test.app" {
		t.Errorf("expected bundleId 'com.test.app', got '%s'", resp.BundleId)
	}
}

func TestGetAllSubscriptionStatuses_Unauthorized(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}

	client := newTestClient(t, handler)
	_, err := client.GetAllSubscriptionStatuses(context.Background(), "test-tx-id")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
}

func TestGetRefundHistory_Success(t *testing.T) {
	expected := RefundHistoryResponse{
		HasMore:            false,
		Revision:           "rev1",
		SignedTransactions: []string{},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}

	client := newTestClient(t, handler)
	resp, err := client.GetRefundHistory(context.Background(), "test-tx-id", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HasMore {
		t.Error("expected hasMore to be false")
	}
}

func TestGetRefundHistory_WithRevision(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		revision := r.URL.Query().Get("revision")
		if revision != "rev456" {
			t.Errorf("expected revision 'rev456', got '%s'", revision)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RefundHistoryResponse{})
	}

	client := newTestClient(t, handler)
	_, err := client.GetRefundHistory(context.Background(), "test-tx-id", "rev456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
