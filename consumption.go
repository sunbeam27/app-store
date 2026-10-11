package app_store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DeliveryStatus describes delivery of purchased content (consumption API V2).
type DeliveryStatus string

const (
	DeliveryStatusDelivered               DeliveryStatus = "DELIVERED"
	DeliveryStatusUndeliveredQualityIssue DeliveryStatus = "UNDELIVERED_QUALITY_ISSUE"
	DeliveryStatusUndeliveredWrongItem    DeliveryStatus = "UNDELIVERED_WRONG_ITEM"
	DeliveryStatusUndeliveredServerOutage DeliveryStatus = "UNDELIVERED_SERVER_OUTAGE"
	DeliveryStatusUndeliveredOther        DeliveryStatus = "UNDELIVERED_OTHER"
)

type RefundPreference string

const (
	RefundPreferenceDecline       RefundPreference = "DECLINE"
	RefundPreferenceGrantFull     RefundPreference = "GRANT_FULL"
	RefundPreferenceGrantProrated RefundPreference = "GRANT_PRORATED"
)

// ConsumptionRequest is the body for Send Consumption Information V2.
// https://developer.apple.com/documentation/appstoreserverapi/consumptionrequest
// Percentage is in millipercent: 25000 means 25%, 100000 means 100%.
// Nil omits the percentage; a pointer to zero explicitly sends 0.
// For auto-renewable subscriptions omit the percentage, including for prorated
// refunds. Other product types require a percentage strictly between 0 and
// 100000 for a prorated preference. Callers must enforce product-specific rules.
type ConsumptionRequest struct {
	CustomerConsented     bool             `json:"customerConsented"`
	ConsumptionPercentage *int32           `json:"consumptionPercentage,omitempty"`
	DeliveryStatus        DeliveryStatus   `json:"deliveryStatus"`
	RefundPreference      RefundPreference `json:"refundPreference,omitempty"`
	SampleContentProvided bool             `json:"sampleContentProvided"`
}

func (r ConsumptionRequest) validate() error {
	if !r.CustomerConsented {
		return errors.New("customer consent is required to send consumption information")
	}
	switch r.DeliveryStatus {
	case DeliveryStatusDelivered, DeliveryStatusUndeliveredQualityIssue, DeliveryStatusUndeliveredWrongItem, DeliveryStatusUndeliveredServerOutage, DeliveryStatusUndeliveredOther:
	default:
		return errors.New("invalid delivery status")
	}
	switch r.RefundPreference {
	case "", RefundPreferenceDecline, RefundPreferenceGrantFull, RefundPreferenceGrantProrated:
	default:
		return errors.New("invalid refund preference")
	}
	if r.ConsumptionPercentage != nil {
		percentage := *r.ConsumptionPercentage
		if percentage < 0 || percentage > 100000 {
			return errors.New("consumption percentage must be between 0 and 100000")
		}
		if r.DeliveryStatus != DeliveryStatusDelivered && percentage != 0 {
			return errors.New("undelivered purchases cannot have a nonzero consumption percentage")
		}
	}
	return nil
}

// SendConsumptionInformation replies to a CONSUMPTION_REQUEST using Apple's V2
// API. Use transactionId from the verified transaction, not originalTransactionId
// or notificationUUID. Submit within 12 hours, after obtaining customer consent.
// HTTP 202 acknowledges receipt; it is not a refund decision.
// https://developer.apple.com/documentation/appstoreserverapi/send-consumption-information
func (c *Client) SendConsumptionInformation(ctx context.Context, transactionId string, consumption ConsumptionRequest) error {
	if strings.TrimSpace(transactionId) == "" {
		return errors.New("transaction ID is required")
	}
	if err := consumption.validate(); err != nil {
		return err
	}
	body, err := json.Marshal(consumption)
	if err != nil {
		return fmt.Errorf("encode consumption information: %w", err)
	}
	bearer, err := c.token.bearer()
	if err != nil {
		return fmt.Errorf("sign consumption request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		c.baseURL+"/inApps/v2/transactions/consumption/"+url.PathEscape(transactionId), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", bearer)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send consumption information: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return nil
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read consumption response: %w", err)
	}
	var appleError ErrorResponse
	if json.Unmarshal(responseBody, &appleError) == nil && appleError.ErrorCode != 0 {
		return fmt.Errorf("send consumption information (HTTP %d): %w", resp.StatusCode, &appleError)
	}
	return fmt.Errorf("send consumption information (HTTP %d): %s", resp.StatusCode, responseBody)
}
