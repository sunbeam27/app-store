// Package appstoreconnect manages apps, builds, in-app purchases and subscriptions.
// Its credentials and JSON:API transport are separate from App Store Server API.
package appstoreconnect

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/sunbeam27/app-store/internal/applehttp"
	"github.com/sunbeam27/app-store/internal/applejwt"
)

const BaseURL = "https://api.appstoreconnect.apple.com"
const OpenAPIVersion = "4.5.1"

type APIError = applehttp.APIError
type ErrorDetail = applehttp.ErrorDetail

type Config struct {
	SigningKey string
	KeyID      string
	IssuerID   string
	// IndividualKey uses Apple's individual-key sub=user claim instead of iss.
	IndividualKey bool
	HTTPClient    *http.Client
	BaseURL       string
	TokenProvider func(context.Context) (string, error)
}

type Client struct{ transport *applehttp.Client }

func NewClient(config Config) (*Client, error) {
	base := config.BaseURL
	if base == "" {
		base = BaseURL
	}
	provider := config.TokenProvider
	if provider == nil {
		subject := ""
		if config.IndividualKey {
			if config.IssuerID != "" {
				return nil, errors.New("individual keys must not specify an issuer ID")
			}
			subject = "user"
		}
		signer, err := applejwt.New(config.SigningKey, config.KeyID, config.IssuerID, "", subject)
		if err != nil {
			return nil, err
		}
		provider = signer.Token
	}
	transport, err := applehttp.New(base, config.HTTPClient, provider)
	if err != nil {
		return nil, err
	}
	return &Client{transport: transport}, nil
}

// Do supports additional App Store Connect operations. Paths and pagination URLs
// must use this client's configured origin. No automatic mutation retries occur.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, request, response any) error {
	contentType := ""
	if request != nil {
		contentType = "application/json"
	}
	return c.transport.Do(ctx, method, path, query, request, contentType, response)
}
