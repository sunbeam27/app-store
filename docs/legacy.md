# 旧版根包接口（v0.1 API）

新接入应使用[主 README](../README.md)介绍的新包。根包 API 继续保留，以兼容现有代码。

# app-store

用于 Apple App Store Server API 和 Sign in with Apple 的 Go SDK。

## 功能

- **App Store Server API**：生成 JWT 令牌、查询订单、交易详情和历史、订阅状态、退款历史，以及提交消费信息 V2。
- **Sign in with Apple**：验证 ID Token、交换授权码、刷新和撤销令牌。
- **App Store Server Notifications V2**：解析通知并验证签名。
- **证书验证**：验证 Apple 根证书链。

## 安装

```bash
go get github.com/sunbeam27/app-store@v0.1.0
```

## 发布

SDK 以带版本的 Go module 和源码归档发布，不提供独立二进制文件。申请发布时，在 `main` 上将 `.github/release-version` 更新为新的 `v0.x.y` 或 `v1.x.y`。发布工作流检查依赖，执行 race 测试、vet 和构建，再在已验证的提交上创建标签和 GitHub Release，并检查发布后的 module 能否通过 Go 代理下载。已有版本和标签不会被覆盖，也可在 `main` 上手动重新运行工作流。

## App Store Server API

### 初始化

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

### 查询订单

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

### 查询交易详情

```go
tx, err := client.GetTransactionInfo(ctx, "TRANSACTION_ID")
if err != nil {
    log.Fatal(err)
}
fmt.Println(tx.ProductId, tx.PurchaseDate)
```

### 查询交易历史

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

### 查询订阅状态

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

### 查询退款历史

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

## Sign in with Apple 登录

### 初始化

```go
authClient := appstore.NewAppleAuthClient(
    "com.your.services.id",  // Services ID (or Bundle ID)
    "YOUR_TEAM_ID",
    "YOUR_KEY_ID",
    privateKeyPEM,           // .p8 file content
)
```

### 交换授权码

```go
tokenResp, err := authClient.ExchangeCode("AUTH_CODE", "https://your-app.com/callback")
if err != nil {
    log.Fatal(err)
}
fmt.Println(tokenResp.AccessToken, tokenResp.IDToken)
```

### 验证 ID Token

为每次授权请求生成并保存 nonce，验证 ID Token 时传入预期 nonce。该值必须与发送给 Apple 的值一致，包括平台要求的哈希处理。`VerifyIDToken` 只验证签名和声明；使用 `VerifyIDTokenWithNonce` 将令牌绑定到对应登录请求。

```go
claims, err := authClient.VerifyIDTokenWithNonce(idToken, expectedNonce)
if err != nil {
    log.Fatal(err)
}
fmt.Println(claims.Subject, claims.Email, claims.IsEmailVerified())
```

### 刷新令牌

```go
tokenResp, err := authClient.RefreshAccessToken("REFRESH_TOKEN")
```

### 撤销令牌

```go
err := authClient.RevokeToken("TOKEN", "access_token") // or "refresh_token"
```

## App Store 服务端通知 V2

### 解析通知

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

### 通知类型

| 类型 | 说明 |
|------|------|
| `SUBSCRIBED` | 新建订阅 |
| `DID_RENEW` | 订阅续期 |
| `DID_FAIL_TO_RENEW` | 续期失败 |
| `DID_CHANGE_RENEWAL_STATUS` | 自动续期设置变化 |
| `DID_CHANGE_RENEWAL_PREF` | 订阅方案变化 |
| `EXPIRED` | 订阅到期 |
| `GRACE_PERIOD_EXPIRED` | 宽限期结束 |
| `OFFER_REDEEMED` | 使用促销优惠 |
| `REFUND` | 交易已退款 |
| `REFUND_DECLINED` | 退款申请被拒绝 |
| `REFUND_REVERSED` | 退款被撤销 |
| `REVOKE` | 家庭共享访问权限被撤销 |
| `CONSUMPTION_REQUEST` | 请求消费信息 |
| `ONE_TIME_CHARGE` | 购买消耗型、非消耗型商品或非自动续期订阅 |
| `RENEWAL_EXTENDED` | 续期日期延长 |
| `PRICE_INCREASE` | 涨价通知 |
| `TEST` | 测试通知 |

### 消费信息请求通知与回复

通知的 HTTP POST 请求体为 `{"signedPayload":"<Apple-signed JWS>"}`。验证签名后，消费信息请求通知的结构如下。下面的 JWS 占位符仅用于展示结构，不是有效的签名测试令牌：

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

验证外层通知及其中的签名交易：

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

将已验证的 `bundleId` 和环境与应用配置核对；生产环境还应核对 `appAppleId`，Apple 在沙盒通知中省略该字段。涉及多个主体时，应根据可信的应用、主体配置选择客户端，并使用匹配的生产或沙盒凭据。交易辅助方法检查内外层的 Bundle ID 和环境是否一致，业务租户的选择和认证由调用方负责。

先取得用户共享消费数据的同意，再从应用记录读取实际使用情况。未取得同意时不要提交消费数据。收到通知后应在 12 小时内提交：

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

该方法发送以下请求：

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

生产环境地址：`https://api.storekit.apple.com`。
沙盒环境地址：`https://api.storekit-sandbox.apple.com`。

Apple 接收数据后返回 **202 Accepted**，方法返回 `nil`，此时不会返回退款决定。最终结果通过 `REFUND` 或 `REFUND_DECLINED` 通知发送。

收到 Apple 通知 POST 后，先持久化受理结果，再回复 **200 OK**。建议先保存消费信息任务，由 worker 提交数据；持久化失败时返回错误响应。使用 `notificationUUID` 去重，在 12 小时窗口内重试临时 API 故障。不要将消费信息 JSON 作为通知 POST 的 HTTP 响应体。

消费信息 V2 的交付状态和退款偏好使用字符串值，V1 使用数值字段：

- `consumptionPercentage` 范围为 0～100000，对应 0%～100%。指向零值的指针会发送 `0`，`nil` 会省略该字段。
- 未交付内容使用合适的 `UNDELIVERED_*` 状态，并将消费比例设为零。
- 自动续期订阅省略消费比例，由 Apple 计算。其他商品类型申请 `GRANT_PRORATED` 时，该值必须严格大于 0、小于 100000。
- 退款偏好为 `DECLINE`、`GRANT_FULL`、`GRANT_PRORATED`，最终决定由 Apple 作出。

参考：[提交消费信息](https://developer.apple.com/documentation/appstoreserverapi/send-consumption-information)、[ConsumptionRequest](https://developer.apple.com/documentation/appstoreserverapi/consumptionrequest)。

### 一次性购买通知

`ONE_TIME_CHARGE` 用于报告消耗型、非消耗型商品和非自动续期订阅的购买，生产与沙盒环境均支持。自动续期订阅继续使用 `SUBSCRIBED` 和 `DID_RENEW`。

验证外层签名后，通知内容如下：

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

使用 `transactionId` 对设备购买处理和通知发货统一去重，避免同一笔交易因重复通知而重复发放内容。缺少 `appAccountToken` 时，应使用已有的可信购买关联确定用户，不要猜测归属。

[examples/notifications/handler.go](../examples/notifications/handler.go) 提供可编译的 HTTP 接收示例，包含应用、环境检查和业务回调。将回调接入持久化存储和消费信息 worker 即可。

参考：[notificationType](https://developer.apple.com/documentation/appstoreservernotifications/notificationtype)。

## 错误处理

```go
order, err := client.LookupOrder(ctx, "ORDER_ID")
if err != nil {
    if e := appstore.UnwrapError(err); e != nil {
        fmt.Printf("Apple error: code=%d message=%s\n", e.ErrorCode, e.ErrorMessage)
    }
}
```

## 参考资料

- [App Store Server API](https://developer.apple.com/documentation/appstoreserverapi)
- [Sign in with Apple 登录](https://developer.apple.com/documentation/sign_in_with_apple)
- [App Store 服务端通知 V2](https://developer.apple.com/documentation/appstoreservernotifications)
