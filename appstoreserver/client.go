package appstoreserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/sunbeam27/app-store/appstoreserver/model"
	"github.com/sunbeam27/app-store/internal/applehttp"
	"github.com/sunbeam27/app-store/internal/applejwt"
)

const (
	ProductionBaseURL  = "https://api.storekit.apple.com"
	SandboxBaseURL     = "https://api.storekit-sandbox.apple.com"
	JavaLibraryVersion = "5.2.0"
)

// APIError preserves HTTP status, raw Apple codes, headers, and retry timing.
type APIError = applehttp.APIError

type ClientConfig struct {
	SigningKey  string
	KeyID       string
	IssuerID    string
	BundleID    string
	Environment model.Environment
	HTTPClient  *http.Client
	// BaseURL overrides the environment host, e.g. for tests or a trusted proxy.
	BaseURL string
	// TokenProvider permits externally managed credentials.
	TokenProvider func(context.Context) (string, error)
}

type AppStoreServerAPIClient struct{ transport *applehttp.Client }

func NewAppStoreServerAPIClient(config ClientConfig) (*AppStoreServerAPIClient, error) {
	base := ProductionBaseURL
	switch config.Environment {
	case model.EnvironmentProduction:
	case model.EnvironmentSandbox:
		base = SandboxBaseURL
	default:
		return nil, errors.New("API environment must be Production or Sandbox")
	}
	if config.BaseURL != "" {
		base = config.BaseURL
	}
	provider := config.TokenProvider
	if provider == nil {
		if config.BundleID == "" {
			return nil, errors.New("bundle ID is required")
		}
		signer, err := applejwt.New(config.SigningKey, config.KeyID, config.IssuerID, config.BundleID, "")
		if err != nil {
			return nil, err
		}
		provider = signer.Token
	}
	transport, err := applehttp.New(base, config.HTTPClient, provider)
	if err != nil {
		return nil, err
	}
	return &AppStoreServerAPIClient{transport: transport}, nil
}

func (c *AppStoreServerAPIClient) call(ctx context.Context, method, path string, query url.Values, body, output any) error {
	contentType := ""
	if body != nil {
		contentType = "application/json"
	}
	return c.transport.Do(ctx, method, path, query, body, contentType, output)
}
