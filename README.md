# app-store

An independently maintained Go SDK for Apple commerce and App Store Connect automation.

| Package | Responsibility |
|---|---|
| `appstoreserver` | Go counterpart to Apple App Store Server Library: 30 API operations, authenticated clients and signed-data verification |
| `appstoreserver/model` | Server API, notifications, retention messaging and Advanced Commerce wire models and enums |
| `appstoreserver/offers` | Legacy promotional offers, promotional offer V2 and introductory eligibility signatures |
| `appstoreserver/advancedcommerce` | StoreKit Advanced Commerce in-app request signatures |
| `appstoreserver/receipt` | Extract transaction IDs when migrating legacy receipts |
| `appstoreconnect` | 52 typed operations for apps, builds, uploads, in-app purchases, subscriptions, localization, availability and pricing |
| `appstoreconnect/model` | App Store Connect JSON:API request and response models |
| `signin` | Sign in with Apple, including request-context variants |

Compatibility baselines: [Apple Java Library 5.2.0](https://github.com/apple/app-store-server-library-java/tree/v5.2.0) and [App Store Connect OpenAPI 4.5.1](https://developer.apple.com/documentation/appstoreconnectapi).
See [compatibility and migration](docs/compatibility.md), [Connect API coverage](docs/connect-api.md), and [compiling examples](examples/sdk/examples.go).

## Installation

Requires **Go 1.26 or later**. This minimum follows the current `golang.org/x/crypto` OCSP dependency.

```sh
go get github.com/sunbeam27/app-store@latest
```

The new packages are part of the next release. The published `v0.1.0` provides the legacy root package; during development, install this branch:

```sh
go get github.com/sunbeam27/app-store@codex/apple-sdk-and-connect
```

The original root API remains available. Its documentation is in [docs/legacy.md](docs/legacy.md).

## App Store Server API

Configure the In-App Purchase signing key, key ID, issuer ID, bundle ID and environment:

```go
import (
    "github.com/sunbeam27/app-store/appstoreserver"
    "github.com/sunbeam27/app-store/appstoreserver/model"
)

client, err := appstoreserver.NewAppStoreServerAPIClient(appstoreserver.ClientConfig{
    SigningKey:  string(keyPEM),
    KeyID:       keyID,
    IssuerID:    issuerID,
    BundleID:    "com.example.app",
    Environment: model.EnvironmentSandbox,
})
if err != nil {
    return err
}

response, err := client.GetTransactionInfo(ctx, transactionID)
if err != nil {
    return err
}
// response.SignedTransactionInfo retains the original signed JWS.
```

The client also supports order lookup, subscription status filtering, transaction and refund pagination, notifications, renewal-date extensions, consumption V1/V2, App Account Token updates, app transactions and finishing transactions. Retention messaging operations share the same client.

API calls return Apple's wire responses. Verification is explicit and independent.

### Verify transactions and notifications

Download the trusted Apple roots from [Apple PKI](https://www.apple.com/certificateauthority/) and pass their DER or PEM contents:

```go
verifier, err := appstoreserver.NewSignedDataVerifier(appstoreserver.VerifierConfig{
    RootCertificates: [][]byte{appleRootDER},
    BundleID:         "com.example.app",
    Environment:      model.EnvironmentSandbox,
    OnlineChecks:     true,
})
if err != nil {
    return err
}
if response.SignedTransactionInfo == nil {
    return errors.New("missing signed transaction")
}
transaction, err := verifier.VerifyAndDecodeTransaction(ctx, *response.SignedTransactionInfo)
```

For production, also set `AppAppleID: model.Ptr(yourAppAppleID)`.
The verifier checks the certificate chain, Apple signing-certificate extensions, JWS signature and application/environment identifiers applicable to each payload type.

Methods cover transactions, renewal information, notifications, app transactions and retention realtime requests. Online checks verify OCSP status at the current time. Verified chains are cached for up to 15 minutes, bounded by certificate expiry. Offline checks validate certificates at the payload signing time, matching Apple's library. Xcode and LocalTesting are explicit testing environments and still enforce application/environment checks.

For an HTTP notification body, decode its `signedPayload` string and pass it to `VerifyAndDecodeNotification`. The inner `signedTransactionInfo` and `signedRenewalInfo` are separate JWS values; verify each one before using it.

### Transaction-history filters

```go
response, err := client.GetTransactionHistory(ctx, transactionID, revision,
    &model.TransactionHistoryRequest{
        ProductIDs: []string{"com.example.monthly"},
        Revoked:    model.Ptr(false),
        Sort:       model.Ptr(model.TransactionHistoryRequestOrderAscending),
    },
    appstoreserver.GetTransactionHistoryVersionV2,
)
```

Keep the same filters when requesting the next revision. The omitted version defaults to V1, matching the Java overload; explicitly select V2 for current history.

## App Store Connect

Configure an App Store Connect integration key separately from the Server API client:

```go
import (
    "github.com/sunbeam27/app-store/appstoreconnect"
    connectmodel "github.com/sunbeam27/app-store/appstoreconnect/model"
)

connect, err := appstoreconnect.NewClient(appstoreconnect.Config{
    SigningKey: string(connectKeyPEM),
    KeyID:      connectKeyID,
    IssuerID:   connectIssuerID,
})
if err != nil {
    return err
}
```

Individual keys use `IndividualKey: true` with an omitted issuer ID. Both clients support custom HTTP clients and external token providers.

### Create an in-app purchase

```go
request := appstoreconnect.NewInAppPurchaseRequest(
    appID, "100 Coins", "com.example.coins100", connectmodel.InAppPurchaseTypeConsumable,
)
product, err := connect.CreateInAppPurchase(ctx, request)
```

This creates the product record. Configure localization, availability and pricing with the corresponding APIs. Submission for review is a separate operation.

### Create an auto-renewable subscription

```go
group, err := connect.CreateSubscriptionGroup(ctx,
    appstoreconnect.NewSubscriptionGroupRequest(appID, "Premium"))
if err != nil {
    return err
}
request := appstoreconnect.NewSubscriptionRequest(
    group.Data.ID, "Premium Monthly", "com.example.premium.monthly",
)
request.Data.Attributes.SubscriptionPeriod = connectmodel.Ptr("ONE_MONTH")
subscription, err := connect.CreateSubscription(ctx, request)
```

Auto-renewable subscriptions belong to a subscription group. They have separate APIs from one-time in-app purchases.

### Upload an IPA or PKG

The SDK uses Apple's [Build Uploads API](https://developer.apple.com/documentation/appstoreconnectapi/post-v1-builduploads): create the upload, reserve each file, stream the specified byte ranges, and commit each file.

```go
file, err := os.Open("MyApp.ipa")
if err != nil {
    return err
}
defer file.Close()
info, err := file.Stat()
if err != nil {
    return err
}

upload, err := connect.UploadBuild(ctx,
    appstoreconnect.NewBuildUploadRequest(appID, "1.0", "42", connectmodel.PlatformIos),
    []appstoreconnect.UploadFile{{
        Name:    info.Name(),
        Size:    info.Size(),
        Content: file,
    }},
)
if err != nil {
    return err
}
// File commitment starts asynchronous processing.
processed, err := connect.WaitForBuildUpload(ctx, upload.Data.ID, 10*time.Second)
```

Use a context deadline to bound uploads and processing waits. `.ipa` and `.pkg` UTIs are inferred; other supported asset types take an explicit UTI. Checksum objects are optional and can be provided through `UploadFile.Checksums`.

`UploadBuildFile`, `CreateBuildUploadFile`, `UploadOperations`, `CommitBuildUploadFile` and status APIs are also available separately. Failed uploads are never automatically committed. `UploadError` retains upload/file IDs, and the high-level uploader returns an existing reservation on later failure. Upload URLs receive only Apple's operation headers, without the Connect API bearer token. The SDK does not automatically retry mutations.

### Filters and pagination

GET methods accept `url.Values` for filters, sparse fields, included resources and limits:

```go
apps, err := connect.ListApps(ctx, url.Values{
    "filter[bundleId]": {"com.example.app"},
    "limit":           {"50"},
})
```

Pass `response.Links.Next` to `Client.Do` for the next page. Additional JSON API operations can also use `Do`; authenticated requests stay on the configured API origin.

## Sign in with Apple

```go
import "github.com/sunbeam27/app-store/signin"

login := signin.NewAppleAuthClient(clientID, teamID, keyID, string(signInKeyPEM))
tokens, err := login.ExchangeCodeContext(ctx, authorizationCode, redirectURI)
claims, err := login.VerifyIDTokenWithNonceContext(ctx, idToken, expectedNonce)
```

The package also supports refresh and revocation with context variants. Existing methods remain available for compatibility.

## Models and errors

Optional scalar fields use pointers so missing values differ from zero and false. Typed enums preserve unknown string/numeric values, and models retain unknown JSON fields in `UnknownFields`. JSON:API `included` unions use `json.RawMessage`.

Use `errors.As` with `*appstoreserver.APIError` or `*appstoreconnect.APIError` to inspect HTTP status, response headers, retry timing and the original body. Server errors retain the raw Apple error code; Connect errors retain the JSON:API error array and source locations. Verification errors expose a `VerificationStatus` and distinguish retryable OCSP failures.

## Development and releases

```sh
go test -race ./...
go vet ./...
go build ./...
```

Model generation is reproducible from pinned upstream inputs; see [docs/compatibility.md](docs/compatibility.md). Tests include Apple JSON, signed-data, Xcode and receipt vectors, synthetic certificate/OCSP cases and a complete mock multipart upload.

Releases remain versioned Go modules and source archives. Updating `.github/release-version` on `main` requests a release after dependency, race, vet and build checks. Existing tags/releases are never overwritten.
