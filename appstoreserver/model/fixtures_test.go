package model_test

import (
	"bytes"
	"encoding/json"
	"github.com/sunbeam27/app-store/appstoreserver/model"
	"os"
	"reflect"
	"testing"
)

func TestAppleModelFixtures(t *testing.T) {
	cases := []struct {
		file  string
		value any
	}{
		{"advancedCommerceDescriptors.json", &model.AdvancedCommerceDescriptors{}},
		{"advancedCommerceOffer.json", &model.AdvancedCommerceOffer{}},
		{"advancedCommerceOneTimeChargeCreateRequest.json", &model.AdvancedCommerceOneTimeChargeCreateRequest{}},
		{"advancedCommerceOneTimeChargeItem.json", &model.AdvancedCommerceOneTimeChargeItem{}},
		{"advancedCommerceRequestInfo.json", &model.AdvancedCommerceRequestInfo{}},
		{"advancedCommerceRequestRefundItem.json", &model.AdvancedCommerceRequestRefundItem{}},
		{"advancedCommerceRequestRefundRequest.json", &model.AdvancedCommerceRequestRefundRequest{}},
		{"advancedCommerceRequestRefundResponse.json", &model.AdvancedCommerceRequestRefundResponse{}},
		{"advancedCommerceSubscriptionCancelRequest.json", &model.AdvancedCommerceSubscriptionCancelRequest{}},
		{"advancedCommerceSubscriptionCancelResponse.json", &model.AdvancedCommerceSubscriptionCancelResponse{}},
		{"advancedCommerceSubscriptionChangeMetadataDescriptors.json", &model.AdvancedCommerceSubscriptionChangeMetadataDescriptors{}},
		{"advancedCommerceSubscriptionChangeMetadataItem.json", &model.AdvancedCommerceSubscriptionChangeMetadataItem{}},
		{"advancedCommerceSubscriptionChangeMetadataRequest.json", &model.AdvancedCommerceSubscriptionChangeMetadataRequest{}},
		{"advancedCommerceSubscriptionChangeMetadataResponse.json", &model.AdvancedCommerceSubscriptionChangeMetadataResponse{}},
		{"advancedCommerceSubscriptionCreateItem.json", &model.AdvancedCommerceSubscriptionCreateItem{}},
		{"advancedCommerceSubscriptionCreateRequest.json", &model.AdvancedCommerceSubscriptionCreateRequest{}},
		{"advancedCommerceSubscriptionMigrateDescriptors.json", &model.AdvancedCommerceSubscriptionMigrateDescriptors{}},
		{"advancedCommerceSubscriptionMigrateItem.json", &model.AdvancedCommerceSubscriptionMigrateItem{}},
		{"advancedCommerceSubscriptionMigrateRenewalItem.json", &model.AdvancedCommerceSubscriptionMigrateRenewalItem{}},
		{"advancedCommerceSubscriptionMigrateRequest.json", &model.AdvancedCommerceSubscriptionMigrateRequest{}},
		{"advancedCommerceSubscriptionMigrateResponse.json", &model.AdvancedCommerceSubscriptionMigrateResponse{}},
		{"advancedCommerceSubscriptionModifyAddItem.json", &model.AdvancedCommerceSubscriptionModifyAddItem{}},
		{"advancedCommerceSubscriptionModifyChangeItem.json", &model.AdvancedCommerceSubscriptionModifyChangeItem{}},
		{"advancedCommerceSubscriptionModifyDescriptors.json", &model.AdvancedCommerceSubscriptionModifyDescriptors{}},
		{"advancedCommerceSubscriptionModifyInAppRequest.json", &model.AdvancedCommerceSubscriptionModifyInAppRequest{}},
		{"advancedCommerceSubscriptionModifyPeriodChange.json", &model.AdvancedCommerceSubscriptionModifyPeriodChange{}},
		{"advancedCommerceSubscriptionModifyRemoveItem.json", &model.AdvancedCommerceSubscriptionModifyRemoveItem{}},
		{"advancedCommerceSubscriptionPriceChangeItem.json", &model.AdvancedCommerceSubscriptionPriceChangeItem{}},
		{"advancedCommerceSubscriptionPriceChangeRequest.json", &model.AdvancedCommerceSubscriptionPriceChangeRequest{}},
		{"advancedCommerceSubscriptionPriceChangeResponse.json", &model.AdvancedCommerceSubscriptionPriceChangeResponse{}},
		{"advancedCommerceSubscriptionReactivateInAppRequest.json", &model.AdvancedCommerceSubscriptionReactivateInAppRequest{}},
		{"advancedCommerceSubscriptionReactivateItem.json", &model.AdvancedCommerceSubscriptionReactivateItem{}},
		{"advancedCommerceSubscriptionRevokeRequest.json", &model.AdvancedCommerceSubscriptionRevokeRequest{}},
		{"advancedCommerceSubscriptionRevokeResponse.json", &model.AdvancedCommerceSubscriptionRevokeResponse{}},
		{"appData.json", &model.AppData{}},
		{"appTransaction.json", &model.AppTransaction{}},
		{"appTransactionInfoResponse.json", &model.AppTransactionInfoResponse{}},
		{"decodedRealtimeRequest.json", &model.DecodedRealtimeRequestBody{}},
		{"extendRenewalDateForAllActiveSubscribersResponse.json", &model.MassExtendRenewalDateResponse{}},
		{"extendSubscriptionRenewalDateResponse.json", &model.ExtendRenewalDateResponse{}},
		{"getAllSubscriptionStatusesResponse.json", &model.StatusResponse{}},
		{"getImageListResponse.json", &model.GetImageListResponse{}},
		{"getMessageListResponse.json", &model.GetMessageListResponse{}},
		{"getNotificationHistoryResponse.json", &model.NotificationHistoryResponse{}},
		{"getRefundHistoryResponse.json", &model.RefundHistoryResponse{}},
		{"getStatusOfSubscriptionRenewalDateExtensionsResponse.json", &model.MassExtendRenewalDateStatusResponse{}},
		{"getTestNotificationStatusResponse.json", &model.CheckTestNotificationResponse{}},
		{"lookupOrderIdResponse.json", &model.OrderLookupResponse{}},
		{"performanceTestResponse.json", &model.PerformanceTestResponse{}},
		{"performanceTestResultResponse.json", &model.PerformanceTestResultResponse{}},
		{"requestTestNotificationResponse.json", &model.SendTestNotificationResponse{}},
		{"signedConsumptionRequestNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedExternalPurchaseTokenNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedExternalPurchaseTokenSandboxNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedRenewalInfo.json", &model.JWSRenewalInfoDecodedPayload{}},
		{"signedRescindConsentNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedSummaryNotification.json", &model.ResponseBodyV2DecodedPayload{}},
		{"signedTransaction.json", &model.JWSTransactionDecodedPayload{}},
		{"signedTransactionWithRevocation.json", &model.JWSTransactionDecodedPayload{}},
		{"transactionHistoryResponse.json", &model.HistoryResponse{}},
		{"transactionInfoResponse.json", &model.TransactionInfoResponse{}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile("../testdata/apple/models/" + tc.file)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, tc.value); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.UseNumber()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			decoder = json.NewDecoder(bytes.NewReader(encoded))
			decoder.UseNumber()
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			assertFields(t, want, got)
		})
	}
}
func assertFields(t *testing.T, want, got any) {
	t.Helper()
	if want == nil {
		return
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected object, got %T", got)
		}
		for key, value := range w {
			if value != nil {
				assertFields(t, value, g[key])
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			t.Fatalf("array length: %v vs %v", w, got)
		}
		for i, value := range w {
			assertFields(t, value, g[i])
		}
	default:
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("field changed: %v (%T) vs %v (%T)", want, want, got, got)
		}
	}
}
