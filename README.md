# app-store

Go SDK for Apple App Store Server API and Sign in with Apple.

## Features

- **App Store Server API** - JWT token generation, order lookup, transaction info/history, subscription status, refund history, consumption information V2
- **Sign in with Apple** - ID token verification, authorization code exchange, token refresh, token revocation
- **App Store Server Notifications V2** - Parse and verify signed notifications
- **Certificate Verification** - Apple root certificate chain validation

## Installation

```bash
go get github.com/sunbeam27/app-store
```

## App Store Server API

### Setup

```go
import appstore "github.com/sunbeam27/app-store"

// Parse your private key (.p8 file)
privateKey, err := appstore.ParsePrivateKey(privateKeyPEM)

// Create token
token := appstore.NewToken("YOUR_KEY_ID", "YOUR_ISSUER_ID", "com.your.bundle", privateKey)

// Create client (production)
client, err := appstore.NewClient(token)

// Or create client for sandbox
client, err := appstore.NewClient(token, appstore.WithSandbox())

// With custom HTTP client
client, err := appstore.NewClient(token, appstore.WithHttpClient(customHTTPClient))
```

### Lookup Order

```go
order, err := client.LookupOrder(ctx, "ORDER_ID")
if err != nil {
    log.Fatal(err)
}

// Decode signed transactions
err = order.DecodeTransaction()
for _, tx := range order.Transactions {
    fmt.Println(tx.ProductId, tx.TransactionId)
}
```

### Get Transaction Info

```go
tx, err := client.GetTransactionInfo(ctx, "TRANSACTION_ID")
if err != nil {
    log.Fatal(err)
}
fmt.Println(tx.ProductId, tx.PurchaseDate)
```

### Get Transaction History

```go
resp, err := client.GetTransactionHistory(ctx, "ORIGINAL_TRANSACTION_ID", "")
if err != nil {
    log.Fatal(err)
}

// Paginate with revision
for resp.HasMore {
    resp, err = client.GetTransactionHistory(ctx, "ORIGINAL_TRANSACTION_ID", resp.Revision)
}
```

### Get Subscription Status

```go
status, err := client.GetAllSubscriptionStatuses(ctx, "ORIGINAL_TRANSACTION_ID")
if err != nil {
    log.Fatal(err)
}

for _, group := range status.Data {
    for _, item := range group.LastTransactions {
        fmt.Printf("Status: %d\n", item.Status)
    }
}
```

### Get Refund History

```go
refunds, err := client.GetRefundHistory(ctx, "ORIGINAL_TRANSACTION_ID", "")
if err != nil {
    log.Fatal(err)
}

// Paginate
for refunds.HasMore {
    refunds, err = client.GetRefundHistory(ctx, "ORIGINAL_TRANSACTION_ID", refunds.Revision)
}
```

## Sign in with Apple

### Setup

```go
authClient := appstore.NewAppleAuthClient(
    "com.your.services.id",  // Services ID (or Bundle ID)
    "YOUR_TEAM_ID",
    "YOUR_KEY_ID",
    privateKeyPEM,           // .p8 file content
)
```

### Exchange Authorization Code

```go
tokenResp, err := authClient.ExchangeCode("AUTH_CODE", "https://your-app.com/callback")
if err != nil {
    log.Fatal(err)
}
fmt.Println(tokenResp.AccessToken, tokenResp.IDToken)
```

### Verify ID Token

Generate and retain a nonce for each authorization request, then pass the expected nonce when verifying its ID token. The nonce must match the value sent to Apple (including any hashing required by your platform). `VerifyIDToken` validates the signature and claims only; use `VerifyIDTokenWithNonce` to bind the token to the login request.

```go
claims, err := authClient.VerifyIDTokenWithNonce(idToken, expectedNonce)
if err != nil {
    log.Fatal(err)
}
fmt.Println(claims.Subject, claims.Email, claims.IsEmailVerified())
```

### Refresh Token

```go
tokenResp, err := authClient.RefreshAccessToken("REFRESH_TOKEN")
```

### Revoke Token

```go
err := authClient.RevokeToken("TOKEN", "access_token") // or "refresh_token"
```

## App Store Server Notifications V2

### Parse Notification

```go
// From raw HTTP request body
payload, err := appstore.ParseNotificationRequest(body)
if err != nil {
    log.Fatal(err)
}

fmt.Println(payload.NotificationType, payload.Subtype)

// Decode transaction and renewal info
if payload.Data != nil {
    tx, err := payload.Data.DecodeTransactionInfo()
    renewal, err := payload.Data.DecodeRenewalInfo()
}
```

### Notification Types

| Type | Description |
|------|-------------|
| `SUBSCRIBED` | New subscription |
| `DID_RENEW` | Subscription renewed |
| `DID_FAIL_TO_RENEW` | Renewal failed |
| `DID_CHANGE_RENEWAL_STATUS` | Auto-renew preference changed |
| `DID_CHANGE_RENEWAL_PREF` | Subscription plan changed |
| `EXPIRED` | Subscription expired |
| `GRACE_PERIOD_EXPIRED` | Grace period ended |
| `OFFER_REDEEMED` | Promotional offer redeemed |
| `REFUND` | Transaction refunded |
| `REFUND_DECLINED` | Refund request declined |
| `REFUND_REVERSED` | Refund reversed |
| `REVOKE` | Family Sharing access revoked |
| `CONSUMPTION_REQUEST` | Consumption info requested |
| `ONE_TIME_CHARGE` | Consumable, non-consumable, or non-renewing subscription purchase |
| `RENEWAL_EXTENDED` | Renewal date extended |
| `PRICE_INCREASE` | Price increase notification |
| `TEST` | Test notification |

### Consumption Request Notification and Reply

The notification's HTTP POST body is `{"signedPayload":"<Apple-signed JWS>"}`.
After signature verification, a consumption notification has this shape
(the JWS placeholders below are illustrative, not valid signed test tokens):

```json
{
  "notificationType": "CONSUMPTION_REQUEST",
  "notificationUUID": "notification-uuid",
  "version": "2.0",
  "signedDate": 1760000000000,
  "data": {
    "appAppleId": 1234567890,
    "bundleId": "com.example.app",
    "environment": "Production",
    "consumptionRequestReason": "FULFILLMENT_ISSUE",
    "signedTransactionInfo": "<Apple-signed transaction JWS>"
  }
}
```

Verify the outer notification and its signed transaction:

```go
notification, err := appstore.ParseNotificationRequest(body)
if err != nil {
    return err
}
tx, err := notification.ConsumptionRequestTransaction()
if err != nil {
    return err
}
// tx.TransactionId identifies the purchase to report.
// notification.Data.ConsumptionRequestReason contains the refund reason.
```

Check the verified `bundleId` and environment against your app configuration,
and `appAppleId` in production (Apple omits it in sandbox notifications).
For multiple subjects, choose the client from your trusted
app/subject configuration, using the matching production or sandbox credentials.
The transaction helper checks that the inner and outer bundle/environment agree;
it does not select or authenticate your business tenant.

Obtain the customer's consent for sharing consumption data and load actual
usage from your application. Do not submit consumption data without that consent.
Submit within 12 hours of receiving the notification:

```go
// Example: a delivered consumable, with 25% consumed.
// customerConsented and consumedPercentage must come from your own records.
consumedPercentage := int32(25000)
request := appstore.ConsumptionRequest{
    CustomerConsented:     customerConsented,
    ConsumptionPercentage: &consumedPercentage,
    DeliveryStatus:        appstore.DeliveryStatusDelivered,
    SampleContentProvided: false,
    // Omit RefundPreference to express no preference.
}
err = client.SendConsumptionInformation(ctx, tx.TransactionId, request)
if err != nil {
    return err
}
```

The method sends:

```http
PUT /inApps/v2/transactions/consumption/{transactionId}
Authorization: Bearer <App Store Server API JWT>
Content-Type: application/json

{
  "customerConsented": true,
  "consumptionPercentage": 25000,
  "deliveryStatus": "DELIVERED",
  "sampleContentProvided": false
}
```

Production host: `https://api.storekit.apple.com`.
Sandbox host: `https://api.storekit-sandbox.apple.com`.

Apple returns **202 Accepted** when it receives the data. The method returns
`nil`; there is no refund-decision response object. The eventual decision arrives
in a `REFUND` or `REFUND_DECLINED` notification.

Separately, respond to Apple's notification POST with **200 OK** after durable
acceptance. Store a consumption job first, then let a worker submit the data;
return an error response if durable acceptance fails. Deduplicate using
`notificationUUID`, and retry transient API failures within the 12-hour window.
Do not send consumption JSON as the HTTP response to the notification POST.

Consumption V2 uses string delivery/refund values, not V1's numeric fields:

- `consumptionPercentage` ranges from 0 to 100000 (100%). A pointer to zero
  sends `0`; `nil` omits the field.
- For undelivered content, use an appropriate `UNDELIVERED_*` status and zero
  consumption.
- For auto-renewable subscriptions, omit consumption percentage; Apple
  calculates it. Other product types require a value strictly between 0 and
  100000 when requesting `GRANT_PRORATED`.
- Refund preferences are `DECLINE`, `GRANT_FULL`, and `GRANT_PRORATED`.
  Apple makes the final decision.

References: [Send Consumption Information](https://developer.apple.com/documentation/appstoreserverapi/send-consumption-information),
[ConsumptionRequest](https://developer.apple.com/documentation/appstoreserverapi/consumptionrequest).

### One-Time Purchase Notification

`ONE_TIME_CHARGE` reports purchases of consumables, non-consumables, and
non-renewing subscriptions. It is available in production and sandbox.
Auto-renewable subscriptions continue to use `SUBSCRIBED` and `DID_RENEW`.

After outer signature verification, the notification contains:

```json
{
  "notificationType": "ONE_TIME_CHARGE",
  "notificationUUID": "notification-uuid",
  "version": "2.0",
  "signedDate": 1760000000000,
  "data": {
    "appAppleId": 1234567890,
    "bundleId": "com.example.app",
    "environment": "Production",
    "signedTransactionInfo": "<Apple-signed transaction JWS>"
  }
}
```

```go
notification, err := appstore.ParseNotificationRequest(body)
if err != nil {
    return err
}
tx, err := notification.OneTimeChargeTransaction()
if err != nil {
    return err
}
// Validate the app/environment, associate tx.AppAccountToken with your user,
// and atomically record tx.TransactionId plus the content grant.
// Handle repeated notifications and device-originated transactions idempotently.
// Inspect tx.Type, tx.ProductId, tx.Quantity, and tx.InAppOwnerShipType.
```

Use `transactionId` to deduplicate delivery across device purchase processing
and notifications. A notification is not an instruction to grant the same
purchase again. If `appAccountToken` is absent, use an existing verified purchase
association; do not guess the user.

A compiling HTTP receiver with app/environment checks and business callbacks
is provided in [examples/notifications/handler.go](examples/notifications/handler.go).
Connect its callbacks to durable storage and your consumption worker.

Reference: [notificationType](https://developer.apple.com/documentation/appstoreservernotifications/notificationtype).

## Error Handling

```go
order, err := client.LookupOrder(ctx, "ORDER_ID")
if err != nil {
    if e := appstore.UnwrapError(err); e != nil {
        fmt.Printf("Apple error: code=%d message=%s\n", e.ErrorCode, e.ErrorMessage)
    }
}
```

## References

- [App Store Server API](https://developer.apple.com/documentation/appstoreserverapi)
- [Sign in with Apple](https://developer.apple.com/documentation/sign_in_with_apple)
- [App Store Server Notifications V2](https://developer.apple.com/documentation/appstoreservernotifications)

