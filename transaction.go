package app_store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net/http"
	"net/url"
)

const (
	transactionInfoPath    = "/inApps/v1/transactions/%s"
	transactionHistoryPath = "/inApps/v2/history/%s"
)

type TransactionType string

const (
	TransactionTypeAutoRenewable TransactionType = "Auto-Renewable Subscription"
	TransactionTypeNonConsumable TransactionType = "Non-Consumable"
	TransactionTypeConsumable    TransactionType = "Consumable"
	TransactionTypeNonRenewing   TransactionType = "Non-Renewing Subscription"
)

type InAppOwnershipType string

const (
	InAppOwnershipTypeFamilyShared InAppOwnershipType = "FAMILY_SHARED"
	InAppOwnershipTypePurchased    InAppOwnershipType = "PURCHASED"
)

type OfferType int

const (
	OfferTypeIntroductory OfferType = 1
	OfferTypePromotional  OfferType = 2
	OfferTypeSubscription OfferType = 3
)

type Environment string

const (
	EnvironmentSandbox    Environment = "Sandbox"
	EnvironmentProduction Environment = "Production"
)

// JWSTransactionDecodedPayload https://developer.apple.com/documentation/appstoreserverapi/jwstransactiondecodedpayload
type JWSTransactionDecodedPayload struct {
	jwt.RegisteredClaims        `json:"-"`
	AppAccountToken             string          `json:"appAccountToken,omitempty"`
	BundleId                    string          `json:"bundleId,omitempty"`
	Currency                    string          `json:"currency,omitempty"`
	Environment                 Environment     `json:"environment,omitempty"`
	ExpiresDate                 int64           `json:"expiresDate,omitempty"`
	InAppOwnerShipType          string          `json:"inAppOwnershipType,omitempty"`
	IsUpgraded                  bool            `json:"isUpgraded,omitempty"`
	OfferDiscountType           string          `json:"offerDiscountType,omitempty"`
	OfferIdentifier             string          `json:"offerIdentifier,omitempty"`
	OfferType                   OfferType       `json:"offerType,omitempty"`
	OriginalPurchaseDate        int64           `json:"originalPurchaseDate,omitempty"`
	OriginalTransactionId       string          `json:"originalTransactionId,omitempty"`
	Price                       int             `json:"price,omitempty"` // 一个整数值，表示您在 App Store Connect 中配置的 App 内购买或订阅优惠的价格乘以 1000，并在购买时系统记录该值。
	ProductId                   string          `json:"productId,omitempty"`
	PurchaseDate                int64           `json:"purchaseDate,omitempty"` // App Store 在客户帐户中收取购买、恢复产品、订阅或续订费用的 UNIX 时间（以毫秒为单位）。
	Quantity                    int             `json:"quantity,omitempty"`
	RevocationDate              int64           `json:"revocationDate,omitempty"`
	RevocationReason            int32           `json:"revocationReason,omitempty"`
	SignedDate                  int64           `json:"signedDate,omitempty"`
	Storefront                  string          `json:"storefront,omitempty"`
	StorefrontId                string          `json:"storefrontId,omitempty"`
	SubscriptionGroupIdentifier string          `json:"subscriptionGroupIdentifier,omitempty"`
	TransactionId               string          `json:"transactionId,omitempty"`
	TransactionReason           string          `json:"transactionReason,omitempty"`
	Type                        TransactionType `json:"type,omitempty"`
	WebOrderLineItemId          string          `json:"webOrderLineItemId,omitempty"`
}

// TransactionHistoryResponse https://developer.apple.com/documentation/appstoreserverapi/historyresponse
type TransactionHistoryResponse struct {
	AppAppleId         int64    `json:"appAppleId"`
	BundleId           string   `json:"bundleId"`
	Environment        string   `json:"environment"`
	HasMore            bool     `json:"hasMore"`
	Revision           string   `json:"revision"`
	SignedTransactions []string `json:"signedTransactions"`
}

func (c *Client) GetTransactionHistory(ctx context.Context, transactionId string, revision string) (*TransactionHistoryResponse, error) {
	requestURL := c.baseURL + fmt.Sprintf(transactionHistoryPath, transactionId)
	if revision != "" {
		requestURL += "?" + url.Values{"revision": {revision}}.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	bearer, err := c.token.bearer()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", bearer)
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
		var result TransactionHistoryResponse
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

func (c *Client) GetTransactionInfo(ctx context.Context, transactionId string) (*JWSTransactionDecodedPayload, error) {
	bearer, err := c.token.bearer()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf(transactionInfoPath, transactionId), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", bearer)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	jwtTransaction := struct {
		Transaction string `json:"signedTransactionInfo"`
	}{}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		err = json.Unmarshal(bytes, &jwtTransaction)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s", bytes)
	default:
		errRsp := &ErrorResponse{}
		if err = json.Unmarshal(bytes, errRsp); err != nil {
			return nil, err
		}
		return nil, errRsp
	}

	claims := &JWSTransactionDecodedPayload{}
	err = DecodeClaims(jwtTransaction.Transaction, claims)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
