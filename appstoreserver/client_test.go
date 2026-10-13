package appstoreserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunbeam27/app-store/appstoreserver/model"
)

func sdkClient(t *testing.T, handler http.HandlerFunc) *AppStoreServerAPIClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := NewAppStoreServerAPIClient(ClientConfig{Environment: model.EnvironmentSandbox, BaseURL: server.URL, TokenProvider: func(context.Context) (string, error) { return "test-token", nil }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAPIKeepsSignedResponseSeparateFromVerification(t *testing.T) {
	c := sdkClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/inApps/v1/transactions/a%2Fb" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("request: %s", r.URL)
		}
		w.Write([]byte(`{"signedTransactionInfo":"unverified.jws.value","future":true}`))
	})
	r, err := c.GetTransactionInfo(context.Background(), "a/b")
	if err != nil || value(r.SignedTransactionInfo) != "unverified.jws.value" || string(r.UnknownFields["future"]) != "true" {
		t.Fatalf("response: %v %v", r, err)
	}
}

func TestHistoryQueryRetainsFalseAndRepeatedFilters(t *testing.T) {
	c := sdkClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/inApps/v2/history/tx" || q.Get("revoked") != "false" || q.Get("revision") != "a+b &c" || strings.Join(q["productId"], ",") != "one,two" || q.Get("sort") != "DESCENDING" || q.Get("startDate") != "0" {
			t.Errorf("query: %s", r.URL)
		}
		w.Write([]byte(`{"hasMore":false,"revision":"next","signedTransactions":[]}`))
	})
	r, err := c.GetTransactionHistory(context.Background(), "tx", "a+b &c", &model.TransactionHistoryRequest{ProductIDs: []string{"one", "two"}, Revoked: model.Ptr(false), StartDate: model.Ptr(int64(0)), Sort: model.Ptr(model.TransactionHistoryRequestOrderDescending)}, GetTransactionHistoryVersionV2)
	if err != nil || value(r.Revision) != "next" {
		t.Fatal(err)
	}
}

func TestAPIErrorPreservesNonJSONAndRateLimit(t *testing.T) {
	c := sdkClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1760000000000")
		w.WriteHeader(429)
		w.Write([]byte("temporarily unavailable"))
	})
	_, err := c.RequestTestNotification(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.HTTPStatusCode != 429 || api.RetryAfter.UnixMilli() != 1760000000000 || string(api.Body) != "temporarily unavailable" {
		t.Fatalf("error lost context: %v", err)
	}
}

func TestConsumptionAccepts202WithoutResponseBody(t *testing.T) {
	c := sdkClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/inApps/v2/transactions/consumption/tx" {
			t.Errorf("route: %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["consumptionPercentage"] != float64(0) || body["sampleContentProvided"] != false {
			t.Errorf("zero fields missing: %v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	})
	err := c.SendConsumptionInformation(context.Background(), "tx", model.ConsumptionRequest{CustomerConsented: true, DeliveryStatus: model.DeliveryStatusDelivered, ConsumptionPercentage: model.Ptr(int32(0)), SampleContentProvided: false})
	if err != nil {
		t.Fatal(err)
	}
}
