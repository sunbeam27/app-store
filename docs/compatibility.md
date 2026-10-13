# Compatibility and migration

The new packages implement Apple App Store Server Java Library **v5.2.0**, pinned to
`2d50adecdcf4c5cd8fbe285eaad305ef3ef60c17`, and the selected App Store Connect
operations from official OpenAPI **4.5.1**. Compatibility means matching wire
contracts and the documented verification/signature behavior, with Go naming,
contexts and error returns.

## Package mapping

| Apple Java component | Go package / API |
|---|---|
| AppStoreServerAPIClient | `appstoreserver.NewAppStoreServerAPIClient` |
| APIException / APIError | `appstoreserver.APIError` / `APIError*` constants |
| SignedDataVerifier | `appstoreserver.NewSignedDataVerifier` |
| VerificationException | `appstoreserver.VerificationError` |
| model | `appstoreserver/model` |
| PromotionalOfferSignatureCreator | `appstoreserver/offers` |
| PromotionalOfferV2SignatureCreator | `appstoreserver/offers` |
| IntroductoryOfferEligibilitySignatureCreator | `appstoreserver/offers` |
| AdvancedCommerceInAppSignatureCreator | `appstoreserver/advancedcommerce` |
| ReceiptUtility | `appstoreserver/receipt` |

The 30 Server API operations include the Java overloads through optional history
version and image-size arguments. HTTP responses keep the original signed data.
The supported Connect operations are listed in [connect-api.md](connect-api.md).
Connect is a distinct API family with its own client, authentication configuration,
JSON:API errors and resource models.

## Intentional Go adaptations

- Public identifiers use `ID`, `URL`, `UUID`, `SKU` and `UTI` initialisms.
- Network calls and verifier methods take `context.Context`.
- Models are structs with optional scalar pointers, rather than mutable Java builders.
  Required fields are value fields. Configure required values before sending requests;
  Java constructor validations are not replicated as a general Go validation framework.
- Known enums are constants over string/int32 types. New enum values survive decoding.
- Unknown object fields survive round trips. Optional null fields decode as nil and
  may be omitted on re-encoding, as with Apple's nullable model values.
- UUID fields use strings; outbound UUID syntax is the caller's responsibility except
  the nonce argument of the legacy offer signer.
- Xcode timestamp decimals truncate exactly toward zero, including negative dates.
- Verification uses per-instance roots/configuration, with no global mutable trust store.
- Online OCSP checks fail closed and cache successful chains for 15 minutes, capped
  by certificate expiry. Delegated responders must explicitly permit OCSP signing.
- Uploads stream through `io.ReaderAt`, preserve reservation IDs on failure, and
  expose commitment and asynchronous processing as separate steps.

## Migrate from v0.1

The root package remains available, and its public signatures are retained. Sign in
with Apple now lives in `signin`, with root aliases maintaining existing imports.
The minimum toolchain is now **Go 1.26**, required by the current OCSP dependency.

| Legacy usage | New usage |
|---|---|
| ParsePrivateKey + NewToken + NewClient | NewAppStoreServerAPIClient(ClientConfig) |
| WithSandbox() | ClientConfig.Environment = model.EnvironmentSandbox |
| GetTransactionInfo returns decoded payload | GetTransactionInfo returns TransactionInfoResponse; verify its JWS explicitly |
| DecodeClaims / DecodeNotificationV2 | SignedDataVerifier.VerifyAndDecode* |
| ParseNotificationRequest(body) | Decode the signedPayload envelope, then VerifyAndDecodeNotification(ctx, signedPayload) |
| OrderLookupResponse.DecodeTransaction | Verify each original SignedTransactions JWS with the configured verifier |
| AppleAuthClient in root | signin.AppleAuthClient; use *Context methods for cancellation |

The legacy root verifier retains its historical signature-only application binding
behavior. New integrations should use the configured verifier to validate the app
and environment. Notification processing should also verify inner transaction and
renewal JWS values, just as with Apple's library.

## Regenerate the SDK

Obtain the pinned upstream inputs, then run:

```sh
git clone --branch v5.2.0 --depth 1 https://github.com/apple/app-store-server-library-java.git /tmp/apple-java
python3 scripts/generate_models.py server /tmp/apple-java
python3 scripts/generate_server_api.py /tmp/apple-java
python3 scripts/generate_models.py connect scripts/upstream/connect-4.5.1.json
python3 scripts/generate_connect_api.py scripts/upstream/connect-4.5.1.json
```

The generators verify the Java commit and Connect schema version and refuse other
versions. The relevant official OpenAPI subset is retained in
`scripts/upstream/connect-4.5.1.json`, so Connect generation works offline even
after Apple replaces the current download. Deliberately update the source snapshot
and generator baselines after reviewing contract changes. Generators make no
network calls themselves.

Generated definitions carry their source provenance, with Apple license text in
`LICENSE.apple.txt`. Official fixture files are retained in
`appstoreserver/testdata/apple`; update their tests when advancing the baseline.

Validation runs on Go 1.26 and the stable toolchain in CI. It does not require Apple
credentials or mutate any App Store Connect account. Real-account validation of
roles, product review requirements and actual binary processing remains an
integration check for the consuming application.
