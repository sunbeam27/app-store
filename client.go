package app_store

import (
	"fmt"
	"net/http"
)

const (
	ProductionBaseURL = "https://api.storekit.itunes.apple.com"
	SandboxBaseURL    = "https://api.storekit-sandbox.itunes.apple.com"
)

type Client struct {
	token   *Token
	client  *http.Client
	baseURL string
}

func NewClient(token *Token, otp ...Options) (*Client, error) {
	c := &Client{
		token:   token,
		client:  http.DefaultClient,
		baseURL: ProductionBaseURL,
	}
	for _, f := range otp {
		f(c)
	}
	if c.token == nil {
		return nil, fmt.Errorf("token is required")
	}
	return c, nil
}
