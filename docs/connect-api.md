# App Store Connect API coverage

These 52 typed methods follow Apple OpenAPI 4.5.1. Request and response models live in `appstoreconnect/model`.
GET methods accept `url.Values` for filters, includes, field selection and pagination.

| Go method | HTTP | Resource |
|---|---|---|
| `ListApps` | GET | `/v1/apps` |
| `GetApp` | GET | `/v1/apps/{id}` |
| `UpdateApp` | PATCH | `/v1/apps/{id}` |
| `ListAppInAppPurchases` | GET | `/v1/apps/{id}/inAppPurchasesV2` |
| `ListAppSubscriptionGroups` | GET | `/v1/apps/{id}/subscriptionGroups` |
| `ListAppBuildUploads` | GET | `/v1/apps/{id}/buildUploads` |
| `ListBuilds` | GET | `/v1/builds` |
| `GetBuild` | GET | `/v1/builds/{id}` |
| `UpdateBuild` | PATCH | `/v1/builds/{id}` |
| `CreateBuildUpload` | POST | `/v1/buildUploads` |
| `GetBuildUpload` | GET | `/v1/buildUploads/{id}` |
| `DeleteBuildUpload` | DELETE | `/v1/buildUploads/{id}` |
| `ListBuildUploadFiles` | GET | `/v1/buildUploads/{id}/buildUploadFiles` |
| `CreateBuildUploadFile` | POST | `/v1/buildUploadFiles` |
| `GetBuildUploadFile` | GET | `/v1/buildUploadFiles/{id}` |
| `CommitBuildUploadFile` | PATCH | `/v1/buildUploadFiles/{id}` |
| `CreateInAppPurchase` | POST | `/v2/inAppPurchases` |
| `GetInAppPurchase` | GET | `/v2/inAppPurchases/{id}` |
| `UpdateInAppPurchase` | PATCH | `/v2/inAppPurchases/{id}` |
| `DeleteInAppPurchase` | DELETE | `/v2/inAppPurchases/{id}` |
| `ListInAppPurchaseLocalizations` | GET | `/v2/inAppPurchases/{id}/inAppPurchaseLocalizations` |
| `ListInAppPurchasePricePoints` | GET | `/v2/inAppPurchases/{id}/pricePoints` |
| `CreateInAppPurchaseLocalization` | POST | `/v1/inAppPurchaseLocalizations` |
| `GetInAppPurchaseLocalization` | GET | `/v1/inAppPurchaseLocalizations/{id}` |
| `UpdateInAppPurchaseLocalization` | PATCH | `/v1/inAppPurchaseLocalizations/{id}` |
| `DeleteInAppPurchaseLocalization` | DELETE | `/v1/inAppPurchaseLocalizations/{id}` |
| `CreateInAppPurchaseAvailability` | POST | `/v1/inAppPurchaseAvailabilities` |
| `CreateInAppPurchasePriceSchedule` | POST | `/v1/inAppPurchasePriceSchedules` |
| `CreateSubscriptionGroup` | POST | `/v1/subscriptionGroups` |
| `GetSubscriptionGroup` | GET | `/v1/subscriptionGroups/{id}` |
| `UpdateSubscriptionGroup` | PATCH | `/v1/subscriptionGroups/{id}` |
| `DeleteSubscriptionGroup` | DELETE | `/v1/subscriptionGroups/{id}` |
| `ListGroupSubscriptions` | GET | `/v1/subscriptionGroups/{id}/subscriptions` |
| `CreateSubscriptionGroupLocalization` | POST | `/v1/subscriptionGroupLocalizations` |
| `GetSubscriptionGroupLocalization` | GET | `/v1/subscriptionGroupLocalizations/{id}` |
| `UpdateSubscriptionGroupLocalization` | PATCH | `/v1/subscriptionGroupLocalizations/{id}` |
| `DeleteSubscriptionGroupLocalization` | DELETE | `/v1/subscriptionGroupLocalizations/{id}` |
| `CreateSubscription` | POST | `/v1/subscriptions` |
| `GetSubscription` | GET | `/v1/subscriptions/{id}` |
| `UpdateSubscription` | PATCH | `/v1/subscriptions/{id}` |
| `DeleteSubscription` | DELETE | `/v1/subscriptions/{id}` |
| `ListSubscriptionLocalizations` | GET | `/v1/subscriptions/{id}/subscriptionLocalizations` |
| `ListSubscriptionPricePoints` | GET | `/v1/subscriptions/{id}/pricePoints` |
| `ListSubscriptionPrices` | GET | `/v1/subscriptions/{id}/prices` |
| `CreateSubscriptionLocalization` | POST | `/v1/subscriptionLocalizations` |
| `GetSubscriptionLocalization` | GET | `/v1/subscriptionLocalizations/{id}` |
| `UpdateSubscriptionLocalization` | PATCH | `/v1/subscriptionLocalizations/{id}` |
| `DeleteSubscriptionLocalization` | DELETE | `/v1/subscriptionLocalizations/{id}` |
| `CreateSubscriptionAvailability` | POST | `/v1/subscriptionAvailabilities` |
| `CreateSubscriptionPrice` | POST | `/v1/subscriptionPrices` |
| `DeleteSubscriptionPrice` | DELETE | `/v1/subscriptionPrices/{id}` |
| `ListTerritories` | GET | `/v1/territories` |

Use `Client.Do` for additional JSON API operations. It accepts a relative API path or a pagination URL on the configured API origin.
The original JSON:API `included` union values and unknown fields are preserved as `json.RawMessage`.
