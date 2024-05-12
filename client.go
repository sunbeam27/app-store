package app_store

import (
	"fmt"
	"net/http"
)

type Client struct {
	token  *Token
	client *http.Client
}

func NewClient(token *Token, otp ...Options) (*Client, error) {
	c := &Client{
		token:  token,
		client: http.DefaultClient,
	}
	for _, f := range otp {
		f(c)
	}
	if c.token == nil {
		return nil, fmt.Errorf("token is required")
	}
	return c, nil
}
