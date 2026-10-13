package appstoreserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/sunbeam27/app-store/appstoreserver/model"
)

func appleFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/apple/" + path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func appleVerifier(t *testing.T, bundle string, environment model.Environment, appID int64) *SignedDataVerifier {
	t.Helper()
	v, err := NewSignedDataVerifier(VerifierConfig{RootCertificates: [][]byte{appleFixture(t, "certs/testCA.der")}, BundleID: bundle, Environment: environment, AppAppleID: &appID})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func requireStatus(t *testing.T, err error, want VerificationStatus) {
	t.Helper()
	var e *VerificationError
	if !errors.As(err, &e) || e.Status != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}

func TestAppleSignedDataVectors(t *testing.T) {
	ctx := context.Background()
	v := appleVerifier(t, "com.example", model.EnvironmentSandbox, 1234)
	notification, err := v.VerifyAndDecodeNotification(ctx, string(appleFixture(t, "mock_signed_data/testNotification")))
	if err != nil || value(notification.NotificationType) != model.NotificationTypeV2Test {
		t.Fatalf("official notification: %v", err)
	}
	transaction, err := v.VerifyAndDecodeTransaction(ctx, string(appleFixture(t, "mock_signed_data/transactionInfo")))
	if err != nil || value(transaction.Environment) != model.EnvironmentSandbox {
		t.Fatalf("official transaction: %v", err)
	}
	renewal, err := v.VerifyAndDecodeRenewalInfo(ctx, string(appleFixture(t, "mock_signed_data/renewalInfo")))
	if err != nil || value(renewal.Environment) != model.EnvironmentSandbox {
		t.Fatalf("official renewal: %v", err)
	}
	_, err = v.VerifyAndDecodeNotification(ctx, string(appleFixture(t, "mock_signed_data/wrongBundleId")))
	requireStatus(t, err, VerificationInvalidAppIdentifier)
	_, err = v.VerifyAndDecodeNotification(ctx, string(appleFixture(t, "mock_signed_data/missingX5CHeaderClaim")))
	requireStatus(t, err, VerificationFailure)
}

func TestVerifierBindsAppAndEnvironment(t *testing.T) {
	ctx := context.Background()
	signed := string(appleFixture(t, "mock_signed_data/testNotification"))
	_, err := appleVerifier(t, "com.example", model.EnvironmentProduction, 1235).VerifyAndDecodeNotification(ctx, signed)
	requireStatus(t, err, VerificationInvalidAppIdentifier)
	_, err = appleVerifier(t, "com.example", model.EnvironmentProduction, 1234).VerifyAndDecodeNotification(ctx, signed)
	requireStatus(t, err, VerificationInvalidEnvironment)
	_, err = appleVerifier(t, "other.bundle", model.EnvironmentSandbox, 1234).VerifyAndDecodeTransaction(ctx, string(appleFixture(t, "mock_signed_data/transactionInfo")))
	requireStatus(t, err, VerificationInvalidAppIdentifier)
	_, err = NewSignedDataVerifier(VerifierConfig{RootCertificates: [][]byte{appleFixture(t, "certs/testCA.der")}, BundleID: "com.example", Environment: model.EnvironmentProduction})
	if err == nil {
		t.Fatal("production verifier accepted missing app ID")
	}
}

func TestVerifierRejectsTamperingAndMalformedData(t *testing.T) {
	ctx := context.Background()
	v := appleVerifier(t, "com.example", model.EnvironmentSandbox, 1234)
	signed := string(appleFixture(t, "mock_signed_data/transactionInfo"))
	parts := strings.Split(signed, ".")
	var payload map[string]any
	data, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	payload["productId"] = "tampered"
	data, _ = json.Marshal(payload)
	parts[1] = base64.RawURLEncoding.EncodeToString(data)
	_, err := v.VerifyAndDecodeTransaction(ctx, strings.Join(parts, "."))
	requireStatus(t, err, VerificationFailure)
	for _, bad := range []string{"a.b.c.d", "a.b.c", "null.e30.signature"} {
		if _, err := v.VerifyAndDecodeTransaction(ctx, bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = v.VerifyAndDecodeTransaction(ctx, signed)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestXcodeVectorsStillCheckApplication(t *testing.T) {
	ctx := context.Background()
	v := appleVerifier(t, "com.example.naturelab.backyardbirds.example", model.EnvironmentXcode, 1234)
	if _, err := v.VerifyAndDecodeTransaction(ctx, string(appleFixture(t, "xcode/xcode-signed-transaction"))); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyAndDecodeRenewalInfo(ctx, string(appleFixture(t, "xcode/xcode-signed-renewal-info"))); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyAndDecodeAppTransaction(ctx, string(appleFixture(t, "xcode/xcode-signed-app-transaction"))); err != nil {
		t.Fatal(err)
	}
	_, err := appleVerifier(t, "other", model.EnvironmentXcode, 1234).VerifyAndDecodeTransaction(ctx, string(appleFixture(t, "xcode/xcode-signed-transaction")))
	requireStatus(t, err, VerificationInvalidAppIdentifier)
}
