package app_store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

const (
	subscriptionStatusPath = "/inApps/v1/subscriptions/%s"
)

type SubscriptionStatus int32

const (
	SubscriptionStatusActive             SubscriptionStatus = 1
	SubscriptionStatusExpired            SubscriptionStatus = 2
	SubscriptionStatusBillingRetryPeriod SubscriptionStatus = 3
	SubscriptionStatusBillingGracePeriod SubscriptionStatus = 4
	SubscriptionStatusRevoked            SubscriptionStatus = 5
)

type AutoRenewStatus int32

const (
	AutoRenewStatusOff AutoRenewStatus = 0
	AutoRenewStatusOn  AutoRenewStatus = 1
)

type ExpirationIntent int32

const (
	ExpirationIntentCancelled             ExpirationIntent = 1
	ExpirationIntentBillingError          ExpirationIntent = 2
	ExpirationIntentPriceIncreaseDeclined ExpirationIntent = 3
	ExpirationIntentProductUnavailable    ExpirationIntent = 4
	ExpirationIntentOther                 ExpirationIntent = 5
)

// StatusResponse https://developer.apple.com/documentation/appstoreserverapi/statusresponse
type StatusResponse struct {
	BundleId    string                      `json:"bundleId"`
	AppAppleId  int64                       `json:"appAppleId"`
	Environment string                      `json:"environment"`
	Data        []SubscriptionGroupIdentity `json:"data"`
}

// SubscriptionGroupIdentity represents a subscription group.
type SubscriptionGroupIdentity struct {
	SubscriptionGroupIdentifier string                 `json:"subscriptionGroupIdentifier"`
	LastTransactions            []LastTransactionsItem `json:"lastTransactions"`
}

// LastTransactionsItem represents the latest transaction for a subscription.
type LastTransactionsItem struct {
	OriginalTransactionId string             `json:"originalTransactionId"`
	Status                SubscriptionStatus `json:"status"`
	SignedRenewalInfo     string             `json:"signedRenewalInfo"`
	SignedTransactionInfo string             `json:"signedTransactionInfo"`
}

// JWSRenewalInfoDecodedPayload https://developer.apple.com/documentation/appstoreserverapi/jwsrenewalinfodecodedpayload
type JWSRenewalInfoDecodedPayload struct {
	jwt.RegisteredClaims        `json:"-"`
	AutoRenewProductId          string           `json:"autoRenewProductId,omitempty"`
	AutoRenewStatus             AutoRenewStatus  `json:"autoRenewStatus,omitempty"`
	Environment                 Environment      `json:"environment,omitempty"`
	ExpirationIntent            ExpirationIntent `json:"expirationIntent,omitempty"`
	GracePeriodExpiresDate      int64            `json:"gracePeriodExpiresDate,omitempty"`
	IsInBillingRetryPeriod      bool             `json:"isInBillingRetryPeriod,omitempty"`
	OfferIdentifier             string           `json:"offerIdentifier,omitempty"`
	OfferType                   OfferType        `json:"offerType,omitempty"`
	OriginalTransactionId       string           `json:"originalTransactionId,omitempty"`
	PriceIncreaseStatus         int32            `json:"priceIncreaseStatus,omitempty"`
	ProductId                   string           `json:"productId,omitempty"`
	RecentSubscriptionStartDate int64            `json:"recentSubscriptionStartDate,omitempty"`
	SignedDate                  int64            `json:"signedDate,omitempty"`
}

// GetAllSubscriptionStatuses https://developer.apple.com/documentation/appstoreserverapi/get_all_subscription_statuses
func (c *Client) GetAllSubscriptionStatuses(ctx context.Context, transactionId string) (*StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+fmt.Sprintf(subscriptionStatusPath, transactionId), nil)
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
		var result StatusResponse
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
