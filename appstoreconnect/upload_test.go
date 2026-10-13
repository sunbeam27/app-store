package appstoreconnect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunbeam27/app-store/appstoreconnect/model"
)

func connectClient(t *testing.T, server *httptest.Server, httpClient *http.Client) *Client {
	t.Helper()
	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: httpClient, TokenProvider: func(context.Context) (string, error) { return "api-secret", nil }})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestBuildUploadStreamsChunksAndWaitsForProcessing(t *testing.T) {
	var mu sync.Mutex
	chunks := map[string]string{}
	commits := 0
	gets := 0
	blob := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.Header.Get("Authorization") != "" || r.Header.Get("X-Upload-Token") != "operation-token" {
			t.Errorf("blob headers: %v", r.Header)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		chunks[r.URL.Path] = string(body)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer blob.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer api-secret" {
			t.Error("missing API token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/buildUploads":
			var request model.BuildUploadCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Data.Relationships.App.Data.ID != "app" || request.Data.Attributes.CFBundleVersion != "42" {
				t.Errorf("upload request: %+v", request.Data)
			}
			w.WriteHeader(201)
			w.Write([]byte(`{"data":{"type":"buildUploads","id":"upload"}}`))
		case "POST /v1/buildUploadFiles":
			var request model.BuildUploadFileCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Data.Attributes.FileSize != 6 || request.Data.Attributes.UTI != "com.apple.ipa" || request.Data.Relationships.BuildUpload.Data.ID != "upload" {
				t.Errorf("file request: %+v", request.Data)
			}
			operations := []map[string]any{}
			for i, path := range []string{"/first", "/second"} {
				operations = append(operations, map[string]any{"method": "PUT", "url": blob.URL + path, "offset": i * 3, "length": 3, "requestHeaders": []map[string]string{{"name": "X-Upload-Token", "value": "operation-token"}}})
			}
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "buildUploadFiles", "id": "file", "attributes": map[string]any{"uploadOperations": operations}}})
		case "PATCH /v1/buildUploadFiles/file":
			var request model.BuildUploadFileUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request.Data.Attributes == nil || !optional(request.Data.Attributes.Uploaded) {
				t.Error("missing commit marker")
			}
			mu.Lock()
			if chunks["/first"] != "abc" || chunks["/second"] != "def" {
				t.Errorf("wrong chunks: %v", chunks)
			}
			commits++
			mu.Unlock()
			w.Write([]byte(`{"data":{"type":"buildUploadFiles","id":"file","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`))
		case "GET /v1/buildUploads/upload":
			mu.Lock()
			gets++
			state := "PROCESSING"
			if gets >= 3 {
				state = "COMPLETE"
			}
			mu.Unlock()
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "buildUploads", "id": "upload", "attributes": map[string]any{"state": map[string]any{"state": state}}}})
		default:
			t.Errorf("unexpected route %s %s", r.Method, r.URL)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	c := connectClient(t, api, blob.Client())
	response, err := c.UploadBuild(context.Background(), NewBuildUploadRequest("app", "1.0", "42", model.PlatformIos), []UploadFile{{Name: "app.ipa", Size: 6, Content: bytes.NewReader([]byte("abcdef"))}})
	if err != nil || optional(response.Data.Attributes.State.State) != model.BuildUploadStateProcessing {
		t.Fatalf("upload must report processing: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err = c.WaitForBuildUpload(ctx, "upload", time.Millisecond)
	if err != nil || optional(response.Data.Attributes.State.State) != model.BuildUploadStateComplete {
		t.Fatalf("processing: %v", err)
	}
	if commits != 1 {
		t.Fatalf("commit count: %d", commits)
	}
}

func TestFailedUploadKeepsReservationAndDoesNotCommit(t *testing.T) {
	commits := 0
	blob := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer blob.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			commits++
			t.Error("failed upload was committed")
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "buildUploadFiles", "id": "file", "attributes": map[string]any{"uploadOperations": []map[string]any{{"method": "PUT", "url": blob.URL, "length": 3, "offset": 0}}}}})
	}))
	defer api.Close()
	c := connectClient(t, api, blob.Client())
	r, err := c.UploadBuildFile(context.Background(), "upload", UploadFile{Name: "app.ipa", Size: 3, Content: strings.NewReader("abc")})
	var failure *UploadError
	var apiError *APIError
	if r == nil || !errors.As(err, &failure) || failure.UploadID != "upload" || failure.FileID != "file" || !errors.As(err, &apiError) || apiError.HTTPStatusCode != 503 || commits != 0 {
		t.Fatalf("recovery information: %v", err)
	}
}

func TestUploadPlanRejectsInvalidRangesBeforeSending(t *testing.T) {
	c, _ := NewClient(Config{TokenProvider: func(context.Context) (string, error) { t.Fatal("upload must not use API credentials"); return "", nil }})
	for _, operations := range [][]model.DeliveryFileUploadOperation{
		{{Method: model.Ptr("PUT"), URL: model.Ptr("https://example.invalid"), Offset: model.Ptr(int64(1)), Length: model.Ptr(int64(3))}},
		{{Method: model.Ptr("PUT"), URL: model.Ptr("https://example.invalid"), Offset: model.Ptr(int64(0)), Length: model.Ptr(int64(4))}},
		{{Method: model.Ptr("PUT"), URL: model.Ptr("http://example.invalid"), Offset: model.Ptr(int64(0)), Length: model.Ptr(int64(3))}},
	} {
		if err := c.UploadOperations(context.Background(), strings.NewReader("abc"), 3, operations); err == nil {
			t.Fatal("invalid plan accepted")
		}
	}
}

func TestBuildProcessingFailureAndCancellation(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"type":"buildUploads","id":"upload","attributes":{"state":{"state":"FAILED","errors":[{"code":"INVALID_BINARY"}]}}}}`))
	}))
	defer api.Close()
	c := connectClient(t, api, nil)
	_, err := c.WaitForBuildUpload(context.Background(), "upload", time.Millisecond)
	var failure *BuildProcessingError
	if !errors.As(err, &failure) || len(failure.State.Errors) != 1 {
		t.Fatalf("processing details: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.WaitForBuildUpload(ctx, "upload", time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
