# app-store

Go SDK for Apple App Store Server API and Sign in with Apple.

## Features

- **App Store Server API** - JWT token generation, order lookup, transaction info/history, subscription status, refund history
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
| `RENEWAL_EXTENDED` | Renewal date extended |
| `PRICE_INCREASE` | Price increase notification |
| `TEST` | Test notification |

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

