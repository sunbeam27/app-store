// Package notifications demonstrates receiving App Store Server Notifications.
// Connect the callbacks to durable application storage before deploying.
package notifications

import (
	"context"
	appstore "github.com/sunbeam27/app-store"
	"io"
	"net/http"
)

// Handler handles consumption requests and one-time purchases for one app.
// Use a separate configured handler or trusted route for each app/subject.
type Handler struct {
	BundleID   string
	AppAppleID int64
	Clients    map[appstore.Environment]*appstore.Client

	// EnqueueConsumption must durably store the verified notification for a worker
	// to load customer consent and actual usage, then call SendConsumptionInformation
	// within 12 hours. Deduplicate by notificationUUID. Return nil only after durable
	// acceptance (or a recorded decision not to send because consent was not given).
	EnqueueConsumption func(context.Context, *appstore.Client, *appstore.NotificationV2Payload, *appstore.JWSTransactionDecodedPayload) error

	// DeliverPurchase must associate appAccountToken with an existing app user and
	// persist the purchase/content grant atomically. Deduplicate by transactionId
	// across both device receipt processing and notifications. A repeated purchase
	// already handled should return nil, without granting the content again.
	DeliverPurchase func(context.Context, *appstore.JWSTransactionDecodedPayload) error
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if h.BundleID == "" || h.AppAppleID == 0 {
		http.Error(w, "app configuration required", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	notification, err := appstore.ParseNotificationRequest(body)
	if err != nil {
		http.Error(w, "invalid notification", http.StatusBadRequest)
		return
	}
	if notification.Data == nil || notification.Data.BundleId != h.BundleID {
		http.Error(w, "unexpected app", http.StatusBadRequest)
		return
	}
	// Apple omits appAppleId in sandbox notifications.
	if (notification.Data.Environment == appstore.EnvironmentProduction && notification.Data.AppAppleId != h.AppAppleID) ||
		(notification.Data.Environment == appstore.EnvironmentSandbox && notification.Data.AppAppleId != 0 && notification.Data.AppAppleId != h.AppAppleID) {
		http.Error(w, "unexpected app ID", http.StatusBadRequest)
		return
	}
	client := h.Clients[notification.Data.Environment]
	if client == nil {
		http.Error(w, "unsupported environment", http.StatusBadRequest)
		return
	}
	var transaction *appstore.JWSTransactionDecodedPayload
	switch notification.NotificationType {
	case appstore.NotificationTypeConsumptionRequest:
		transaction, err = notification.ConsumptionRequestTransaction()
	case appstore.NotificationTypeOneTimeCharge:
		transaction, err = notification.OneTimeChargeTransaction()
	default:
		// This example only handles these two events. Configure a complete router for
		// your other subscription/refund notification handlers in production.
		http.Error(w, "notification handler not configured", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		http.Error(w, "invalid transaction", http.StatusBadRequest)
		return
	}
	switch notification.NotificationType {
	case appstore.NotificationTypeConsumptionRequest:
		if h.EnqueueConsumption == nil {
			http.Error(w, "consumption handler not configured", http.StatusServiceUnavailable)
			return
		}
		err = h.EnqueueConsumption(r.Context(), client, notification, transaction)
	case appstore.NotificationTypeOneTimeCharge:
		if h.DeliverPurchase == nil {
			http.Error(w, "purchase handler not configured", http.StatusServiceUnavailable)
			return
		}
		err = h.DeliverPurchase(r.Context(), transaction)
	}
	if err != nil {
		http.Error(w, "processing failed; retry later", http.StatusServiceUnavailable)
		return
	}
	// Acknowledge receipt only after the event has been durably accepted.
	// The consumption worker's separate request to Apple receives HTTP 202.
	w.WriteHeader(http.StatusOK)
}
