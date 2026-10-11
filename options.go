package app_store

import "net/http"

type Options func(*Client)

func WithHttpClient(client *http.Client) Options {
	return func(c *Client) {
		c.client = client
	}
}

func WithSandbox() Options {
	return func(c *Client) {
		c.baseURL = SandboxBaseURL
	}
}
