package app_store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	refundHistoryPath = "/inApps/v2/refund/lookup/%s"
)

// RefundHistoryResponse https://developer.apple.com/documentation/appstoreserverapi/refundhistoryresponse
type RefundHistoryResponse struct {
	HasMore            bool     `json:"hasMore"`
	Revision           string   `json:"revision"`
	SignedTransactions []string `json:"signedTransactions"`
}

// GetRefundHistory https://developer.apple.com/documentation/appstoreserverapi/get_refund_history
func (c *Client) GetRefundHistory(ctx context.Context, transactionId string, revision string) (*RefundHistoryResponse, error) {
	url := c.baseURL + fmt.Sprintf(refundHistoryPath, transactionId)
	if revision != "" {
		url += "?revision=" + revision
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.token.Bearer())
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var result RefundHistoryResponse
		if err = json.Unmarshal(body, &result); err != nil {
			return nil, err
		}
		return &result, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s", body)
	default:
		errRsp := &ErrorResponse{}
		if err = json.Unmarshal(body, errRsp); err != nil {
			return nil, err
		}
		return nil, errRsp
	}
}
