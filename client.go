package app_store

import (
	"fmt"
	"net/http"
)

const (
	transactionInfoUrl = "https://api.storekit.itunes.apple.com/inApps/v1/transactions/%s"
)

type Client struct {
	token  *Token
	client *http.Client
}

func NewClient(token *Token) *Client {
	c := &Client{
		token:  token,
		client: http.DefaultClient,
	}
	return c
}

func (c *Client) GetTransactionInfo(transactionId string) *JwsTransactionDecodedPayload {
	bearer := c.token.Bearer()
	req, _ := http.NewRequest("GET", fmt.Sprintf(transactionInfoUrl, transactionId), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearer))
	resp, _ := c.client.Do(req)
	defer resp.Body.Close()

}
