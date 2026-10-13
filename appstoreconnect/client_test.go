package appstoreconnect

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sunbeam27/app-store/appstoreconnect/model"
)

func TestCreateProductsAndSubscriptionRelationships(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		data := body["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		relations := data["relationships"].(map[string]any)
		switch r.URL.Path {
		case "/v2/inAppPurchases":
			if data["type"] != "inAppPurchases" || attrs["productId"] != "coins" || attrs["inAppPurchaseType"] != "CONSUMABLE" || attrs["familySharable"] != false {
				t.Errorf("IAP body: %v", body)
			}
			if relations["app"].(map[string]any)["data"].(map[string]any)["id"] != "app" {
				t.Error("wrong app relationship")
			}
		case "/v1/subscriptions":
			if data["type"] != "subscriptions" || attrs["productId"] != "monthly" || relations["group"].(map[string]any)["data"].(map[string]any)["id"] != "group" {
				t.Errorf("subscription body: %v", body)
			}
		default:
			t.Errorf("unexpected product endpoint %s", r.URL)
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": data["type"], "id": "created"}})
	}))
	defer api.Close()
	c := connectClient(t, api, nil)
	request := NewInAppPurchaseRequest("app", "Coins", "coins", model.InAppPurchaseTypeConsumable)
	request.Data.Attributes.FamilySharable = model.Ptr(false)
	if _, err := c.CreateInAppPurchase(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSubscription(context.Background(), NewSubscriptionRequest("group", "Monthly", "monthly")); err != nil {
		t.Fatal(err)
	}
}

func TestPaginationAndJSONAPIError(t *testing.T) {
	var api *httptest.Server
	api = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "next" {
			w.WriteHeader(409)
			w.Write([]byte(`{"errors":[{"id":"one","code":"ENTITY_ERROR","title":"Conflict","detail":"Duplicate product","source":{"pointer":"/data/attributes/productId"}}]}`))
			return
		}
		if r.URL.Query().Get("filter[bundleId]") != "com.example" {
			t.Error("missing filter")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}, "links": map[string]string{"next": api.URL + "/v1/apps?cursor=next"}})
	}))
	defer api.Close()
	c := connectClient(t, api, nil)
	response, err := c.ListApps(context.Background(), url.Values{"filter[bundleId]": {"com.example"}})
	if err != nil {
		t.Fatal(err)
	}
	if response.Links.Next == nil {
		t.Fatal("missing next page link")
	}
	err = c.Do(context.Background(), "GET", *response.Links.Next, nil, nil, &model.AppsResponse{})
	var failure *APIError
	if !errors.As(err, &failure) || failure.HTTPStatusCode != 409 || len(failure.Errors) != 1 || failure.Errors[0].Code != "ENTITY_ERROR" || len(failure.Errors[0].Source) == 0 {
		t.Fatalf("error lost details: %v", err)
	}
	if err := c.Do(context.Background(), "GET", "https://example.invalid/v1/apps", nil, nil, nil); err == nil {
		t.Fatal("foreign origin accepted")
	}
}

func TestAPIDoesNotFollowRedirects(t *testing.T) {
	forwarded := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer other.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer api.Close()
	c := connectClient(t, api, nil)
	_, err := c.ListApps(context.Background(), nil)
	var failure *APIError
	if !errors.As(err, &failure) || failure.HTTPStatusCode != 302 || forwarded {
		t.Fatalf("redirect leaked API request: %v", err)
	}
}
