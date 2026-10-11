package app_store

import (
	"testing"
)

func TestNotificationTypes(t *testing.T) {
	tests := []struct {
		name     string
		ntype    NotificationType
		expected string
	}{
		{"consumption", NotificationTypeConsumptionRequest, "CONSUMPTION_REQUEST"},
		{"did_renew", NotificationTypeDidRenew, "DID_RENEW"},
		{"expired", NotificationTypeExpired, "EXPIRED"},
		{"refund", NotificationTypeRefund, "REFUND"},
		{"subscribed", NotificationTypeSubscribed, "SUBSCRIBED"},
		{"test", NotificationTypeTest, "TEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.ntype) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.ntype)
			}
		})
	}
}

func TestNotificationSubtypes(t *testing.T) {
	tests := []struct {
		name     string
		subtype  NotificationSubtype
		expected string
	}{
		{"initial_buy", SubtypeInitialBuy, "INITIAL_BUY"},
		{"resubscribe", SubtypeResubscribe, "RESUBSCRIBE"},
		{"upgrade", SubtypeUpgrade, "UPGRADE"},
		{"downgrade", SubtypeDowngrade, "DOWNGRADE"},
		{"voluntary", SubtypeVoluntary, "VOLUNTARY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.subtype) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.subtype)
			}
		})
	}
}

func TestNotificationDataDecodeTransactionInfoEmpty(t *testing.T) {
	data := &NotificationData{}
	result, err := data.DecodeTransactionInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for empty signedTransactionInfo")
	}
}

func TestNotificationDataDecodeRenewalInfoEmpty(t *testing.T) {
	data := &NotificationData{}
	result, err := data.DecodeRenewalInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for empty signedRenewalInfo")
	}
}

func TestDecodeNotificationV2_Invalid(t *testing.T) {
	_, err := DecodeNotificationV2("invalid.jwt.payload")
	if err == nil {
		t.Fatal("expected error for invalid JWT")
	}
}

func TestParseNotificationRequest_InvalidJSON(t *testing.T) {
	_, err := ParseNotificationRequest([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseNotificationRequest_InvalidJWT(t *testing.T) {
	body := []byte(`{"signedPayload":"invalid.jwt.token"}`)
	_, err := ParseNotificationRequest(body)
	if err == nil {
		t.Fatal("expected error for invalid JWT")
	}
}
