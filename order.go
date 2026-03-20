package app_store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	orderPath = "/inApps/v1/lookup/%s"
)

// https://developer.apple.com/documentation/appstoreserverapi/orderlookupresponse
type OrderLookupResponse struct {
	Status             *int32   `json:"status"`             // 0: 有效订单 1: 无效订单
	SignedTransactions []string `json:"signedTransactions"` // JWSTransaction

	// decoded transactions
	Transactions []*JWSTransactionDecodedPayload `json:"-"`
}

func (o *OrderLookupResponse) DecodeTransaction() error {
	if o.Status == nil || *o.Status != 0 {
		return fmt.Errorf("invalid order")
	}
	var errs error
	for _, signedTransaction := range o.SignedTransactions {
		ts := new(JWSTransactionDecodedPayload)
		err := DecodeClaims(signedTransaction, ts)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		o.Transactions = append(o.Transactions, ts)
	}
	return errs
}

func (c *Client) LookupOrder(ctx context.Context, orderId string) (*OrderLookupResponse, error) {
	bearer := c.token.Bearer()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf(orderPath, orderId), nil)
	req.Header.Set("Authorization", bearer)
	resp, _ := c.client.Do(req)
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var order *OrderLookupResponse
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.Unmarshal(bytes, &order)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s", bytes)
	default:
		errRsp := &ErrorResponse{}
		if err = json.Unmarshal(bytes, errRsp); err != nil {
			return nil, err
		}
		return nil, errRsp
	}

	return order, nil
}
