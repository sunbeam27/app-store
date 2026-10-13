# App Store Connect 接口清单

以下 52 个强类型方法以 Apple OpenAPI 4.5.1 为基线，请求和响应模型位于 `appstoreconnect/model`。
GET 方法接收 `url.Values`，用于配置筛选条件、关联资源、返回字段和分页参数。

| Go 方法 | 用途 | HTTP | 资源路径 |
|---|---|---|---|
| `ListApps` | 查询应用列表 | GET | `/v1/apps` |
| `GetApp` | 查询应用详情 | GET | `/v1/apps/{id}` |
| `UpdateApp` | 更新应用信息 | PATCH | `/v1/apps/{id}` |
| `ListAppInAppPurchases` | 查询应用的内购商品 | GET | `/v1/apps/{id}/inAppPurchasesV2` |
| `ListAppSubscriptionGroups` | 查询应用的订阅组 | GET | `/v1/apps/{id}/subscriptionGroups` |
| `ListAppBuildUploads` | 查询应用的构建上传记录 | GET | `/v1/apps/{id}/buildUploads` |
| `ListBuilds` | 查询构建列表 | GET | `/v1/builds` |
| `GetBuild` | 查询构建详情 | GET | `/v1/builds/{id}` |
| `UpdateBuild` | 更新构建信息 | PATCH | `/v1/builds/{id}` |
| `CreateBuildUpload` | 创建构建上传记录 | POST | `/v1/buildUploads` |
| `GetBuildUpload` | 查询构建上传详情和处理状态 | GET | `/v1/buildUploads/{id}` |
| `DeleteBuildUpload` | 删除构建上传记录 | DELETE | `/v1/buildUploads/{id}` |
| `ListBuildUploadFiles` | 查询构建上传的文件列表 | GET | `/v1/buildUploads/{id}/buildUploadFiles` |
| `CreateBuildUploadFile` | 为构建文件预留上传位置 | POST | `/v1/buildUploadFiles` |
| `GetBuildUploadFile` | 查询文件上传详情 | GET | `/v1/buildUploadFiles/{id}` |
| `CommitBuildUploadFile` | 提交已完成上传的文件 | PATCH | `/v1/buildUploadFiles/{id}` |
| `CreateInAppPurchase` | 创建内购商品 | POST | `/v2/inAppPurchases` |
| `GetInAppPurchase` | 查询内购商品详情 | GET | `/v2/inAppPurchases/{id}` |
| `UpdateInAppPurchase` | 更新内购商品 | PATCH | `/v2/inAppPurchases/{id}` |
| `DeleteInAppPurchase` | 删除内购商品 | DELETE | `/v2/inAppPurchases/{id}` |
| `ListInAppPurchaseLocalizations` | 查询商品本地化信息 | GET | `/v2/inAppPurchases/{id}/inAppPurchaseLocalizations` |
| `ListInAppPurchasePricePoints` | 查询商品价格点 | GET | `/v2/inAppPurchases/{id}/pricePoints` |
| `CreateInAppPurchaseLocalization` | 创建商品本地化信息 | POST | `/v1/inAppPurchaseLocalizations` |
| `GetInAppPurchaseLocalization` | 查询商品本地化详情 | GET | `/v1/inAppPurchaseLocalizations/{id}` |
| `UpdateInAppPurchaseLocalization` | 更新商品本地化信息 | PATCH | `/v1/inAppPurchaseLocalizations/{id}` |
| `DeleteInAppPurchaseLocalization` | 删除商品本地化信息 | DELETE | `/v1/inAppPurchaseLocalizations/{id}` |
| `CreateInAppPurchaseAvailability` | 配置商品可售范围 | POST | `/v1/inAppPurchaseAvailabilities` |
| `CreateInAppPurchasePriceSchedule` | 创建商品价格计划 | POST | `/v1/inAppPurchasePriceSchedules` |
| `CreateSubscriptionGroup` | 创建订阅组 | POST | `/v1/subscriptionGroups` |
| `GetSubscriptionGroup` | 查询订阅组详情 | GET | `/v1/subscriptionGroups/{id}` |
| `UpdateSubscriptionGroup` | 更新订阅组 | PATCH | `/v1/subscriptionGroups/{id}` |
| `DeleteSubscriptionGroup` | 删除订阅组 | DELETE | `/v1/subscriptionGroups/{id}` |
| `ListGroupSubscriptions` | 查询订阅组中的订阅 | GET | `/v1/subscriptionGroups/{id}/subscriptions` |
| `CreateSubscriptionGroupLocalization` | 创建订阅组本地化信息 | POST | `/v1/subscriptionGroupLocalizations` |
| `GetSubscriptionGroupLocalization` | 查询订阅组本地化详情 | GET | `/v1/subscriptionGroupLocalizations/{id}` |
| `UpdateSubscriptionGroupLocalization` | 更新订阅组本地化信息 | PATCH | `/v1/subscriptionGroupLocalizations/{id}` |
| `DeleteSubscriptionGroupLocalization` | 删除订阅组本地化信息 | DELETE | `/v1/subscriptionGroupLocalizations/{id}` |
| `CreateSubscription` | 创建自动续期订阅 | POST | `/v1/subscriptions` |
| `GetSubscription` | 查询订阅详情 | GET | `/v1/subscriptions/{id}` |
| `UpdateSubscription` | 更新订阅 | PATCH | `/v1/subscriptions/{id}` |
| `DeleteSubscription` | 删除订阅 | DELETE | `/v1/subscriptions/{id}` |
| `ListSubscriptionLocalizations` | 查询订阅本地化信息 | GET | `/v1/subscriptions/{id}/subscriptionLocalizations` |
| `ListSubscriptionPricePoints` | 查询订阅价格点 | GET | `/v1/subscriptions/{id}/pricePoints` |
| `ListSubscriptionPrices` | 查询订阅价格 | GET | `/v1/subscriptions/{id}/prices` |
| `CreateSubscriptionLocalization` | 创建订阅本地化信息 | POST | `/v1/subscriptionLocalizations` |
| `GetSubscriptionLocalization` | 查询订阅本地化详情 | GET | `/v1/subscriptionLocalizations/{id}` |
| `UpdateSubscriptionLocalization` | 更新订阅本地化信息 | PATCH | `/v1/subscriptionLocalizations/{id}` |
| `DeleteSubscriptionLocalization` | 删除订阅本地化信息 | DELETE | `/v1/subscriptionLocalizations/{id}` |
| `CreateSubscriptionAvailability` | 配置订阅可售范围 | POST | `/v1/subscriptionAvailabilities` |
| `CreateSubscriptionPrice` | 创建订阅价格 | POST | `/v1/subscriptionPrices` |
| `DeleteSubscriptionPrice` | 删除订阅价格 | DELETE | `/v1/subscriptionPrices/{id}` |
| `ListTerritories` | 查询地区列表 | GET | `/v1/territories` |

其他 JSON API 操作可使用 `Client.Do`，传入相对 API 路径，或客户端配置来源地址内的分页 URL。
JSON:API 响应中的 `included` 联合类型和未知字段以 `json.RawMessage` 保留。
