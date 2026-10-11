package app_store

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// Notification types
type NotificationType string

const (
	NotificationTypeConsumptionRequest     NotificationType = "CONSUMPTION_REQUEST"
	NotificationTypeOneTimeCharge          NotificationType = "ONE_TIME_CHARGE"
	NotificationTypeDidChangeRenewalPref   NotificationType = "DID_CHANGE_RENEWAL_PREF"
	NotificationTypeDidChangeRenewalStatus NotificationType = "DID_CHANGE_RENEWAL_STATUS"
	NotificationTypeDidFailToRenew         NotificationType = "DID_FAIL_TO_RENEW"
	NotificationTypeDidRenew               NotificationType = "DID_RENEW"
	NotificationTypeExpired                NotificationType = "EXPIRED"
	NotificationTypeGracePeriodExpired     NotificationType = "GRACE_PERIOD_EXPIRED"
	NotificationTypeOfferRedeemed          NotificationType = "OFFER_REDEEMED"
	NotificationTypePriceIncrease          NotificationType = "PRICE_INCREASE"
	NotificationTypeRefund                 NotificationType = "REFUND"
	NotificationTypeRefundDeclined         NotificationType = "REFUND_DECLINED"
	NotificationTypeRefundReversed         NotificationType = "REFUND_REVERSED"
	NotificationTypeRenewalExtended        NotificationType = "RENEWAL_EXTENDED"
	NotificationTypeRevoke                 NotificationType = "REVOKE"
	NotificationTypeSubscribed             NotificationType = "SUBSCRIBED"
	NotificationTypeTest                   NotificationType = "TEST"
)

// Notification subtypes
type NotificationSubtype string

const (
	SubtypeInitialBuy        NotificationSubtype = "INITIAL_BUY"
	SubtypeResubscribe       NotificationSubtype = "RESUBSCRIBE"
	SubtypeDowngrade         NotificationSubtype = "DOWNGRADE"
	SubtypeUpgrade           NotificationSubtype = "UPGRADE"
	SubtypeAutoRenewEnabled  NotificationSubtype = "AUTO_RENEW_ENABLED"
	SubtypeAutoRenewDisabled NotificationSubtype = "AUTO_RENEW_DISABLED"
	SubtypeVoluntary         NotificationSubtype = "VOLUNTARY"
	SubtypeBillingRetry      NotificationSubtype = "BILLING_RETRY"
	SubtypePriceIncrease     NotificationSubtype = "PRICE_INCREASE"
	SubtypeGracePeriod       NotificationSubtype = "GRACE_PERIOD"
	SubtypePending           NotificationSubtype = "PENDING"
	SubtypeAccepted          NotificationSubtype = "ACCEPTED"
	SubtypeBillingRecovery   NotificationSubtype = "BILLING_RECOVERY"
	SubtypeProductNotForSale NotificationSubtype = "PRODUCT_NOT_FOR_SALE"
	SubtypeSummary           NotificationSubtype = "SUMMARY"
	SubtypeFailure           NotificationSubtype = "FAILURE"
)

// SignedPayload is the request body from Apple's server notification.
type SignedPayload struct {
	SignedPayload string `json:"signedPayload"`
}

// NotificationV2Payload https://developer.apple.com/documentation/appstoreservernotifications/responsebodyv2decodedpayload
type NotificationV2Payload struct {
	jwt.RegisteredClaims `json:"-"`
	NotificationType     NotificationType     `json:"notificationType"`
	Subtype              NotificationSubtype  `json:"subtype,omitempty"`
	NotificationUUID     string               `json:"notificationUUID"`
	Data                 *NotificationData    `json:"data,omitempty"`
	Version              string               `json:"version"`
	SignedDate           int64                `json:"signedDate"`
	Summary              *NotificationSummary `json:"summary,omitempty"`
}

// NotificationData contains the transaction and renewal info.
type NotificationData struct {
	ConsumptionRequestReason ConsumptionRequestReason `json:"consumptionRequestReason,omitempty"`
	AppAppleId               int64                    `json:"appAppleId"`
	BundleId                 string                   `json:"bundleId"`
	BundleVersion            string                   `json:"bundleVersion"`
	Environment              Environment              `json:"environment"`
	SignedRenewalInfo        string                   `json:"signedRenewalInfo"`
	SignedTransactionInfo    string                   `json:"signedTransactionInfo"`
	Status                   int32                    `json:"status"`
}

// ConsumptionRequestReason is the customer's reason for requesting a refund.
type ConsumptionRequestReason string

const (
	ConsumptionRequestReasonUnintendedPurchase      ConsumptionRequestReason = "UNINTENDED_PURCHASE"
	ConsumptionRequestReasonFulfillmentIssue        ConsumptionRequestReason = "FULFILLMENT_ISSUE"
	ConsumptionRequestReasonUnsatisfiedWithPurchase ConsumptionRequestReason = "UNSATISFIED_WITH_PURCHASE"
	ConsumptionRequestReasonLegal                   ConsumptionRequestReason = "LEGAL"
	ConsumptionRequestReasonOther                   ConsumptionRequestReason = "OTHER"
)

// ConsumptionRequestTransaction verifies and decodes the transaction for a
// consumption notification. ParseNotificationRequest must verify the outer JWS
// first; manually constructed payloads are not authenticated by this method.
func (p *NotificationV2Payload) ConsumptionRequestTransaction() (*JWSTransactionDecodedPayload, error) {
	return p.transactionFor(NotificationTypeConsumptionRequest)
}

// OneTimeChargeTransaction verifies and decodes a one-time purchase transaction.
// This event covers consumables, non-consumables and non-renewing subscriptions.
// ParseNotificationRequest must verify the outer JWS first.
func (p *NotificationV2Payload) OneTimeChargeTransaction() (*JWSTransactionDecodedPayload, error) {
	return p.transactionFor(NotificationTypeOneTimeCharge)
}

func (p *NotificationV2Payload) transactionFor(expected NotificationType) (*JWSTransactionDecodedPayload, error) {
	if p == nil || p.NotificationType != expected {
		return nil, fmt.Errorf("expected %s notification", expected)
	}
	if p.Data == nil || p.Data.SignedTransactionInfo == "" {
		return nil, errors.New("notification is missing signed transaction information")
	}
	transaction, err := p.Data.DecodeTransactionInfo()
	if err != nil {
		return nil, fmt.Errorf("decode %s transaction: %w", expected, err)
	}
	if transaction.TransactionId == "" {
		return nil, errors.New("notification transaction is missing transaction ID")
	}
	if transaction.BundleId != p.Data.BundleId || transaction.Environment != p.Data.Environment {
		return nil, errors.New("notification and transaction app or environment mismatch")
	}
	return transaction, nil
}

// NotificationSummary for RENEWAL_EXTENDED summary notifications.
type NotificationSummary struct {
	RequestIdentifier      string      `json:"requestIdentifier"`
	Environment            Environment `json:"environment"`
	AppAppleId             int64       `json:"appAppleId"`
	BundleId               string      `json:"bundleId"`
	ProductId              string      `json:"productId"`
	StorefrontCountryCodes []string    `json:"storefrontCountryCodes"`
	SucceededCount         int64       `json:"succeededCount"`
	FailedCount            int64       `json:"failedCount"`
}

// DecodeNotificationV2 decodes and verifies a V2 server notification.
func DecodeNotificationV2(signedPayload string) (*NotificationV2Payload, error) {
	payload := &NotificationV2Payload{}
	if err := DecodeClaims(signedPayload, payload); err != nil {
		return nil, fmt.Errorf("decode notification: %w", err)
	}
	return payload, nil
}

// DecodeTransactionInfo decodes the signedTransactionInfo from a notification.
func (d *NotificationData) DecodeTransactionInfo() (*JWSTransactionDecodedPayload, error) {
	if d.SignedTransactionInfo == "" {
		return nil, nil
	}
	claims := &JWSTransactionDecodedPayload{}
	if err := DecodeClaims(d.SignedTransactionInfo, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// DecodeRenewalInfo decodes the signedRenewalInfo from a notification.
func (d *NotificationData) DecodeRenewalInfo() (*JWSRenewalInfoDecodedPayload, error) {
	if d.SignedRenewalInfo == "" {
		return nil, nil
	}
	claims := &JWSRenewalInfoDecodedPayload{}
	if err := DecodeClaims(d.SignedRenewalInfo, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// ParseNotificationRequest parses the raw JSON request body into a SignedPayload and decodes it.
func ParseNotificationRequest(body []byte) (*NotificationV2Payload, error) {
	var signed SignedPayload
	if err := json.Unmarshal(body, &signed); err != nil {
		return nil, fmt.Errorf("unmarshal signed payload: %w", err)
	}
	return DecodeNotificationV2(signed.SignedPayload)
}
