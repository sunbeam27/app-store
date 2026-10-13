# app-store

独立维护的 Go SDK，面向 Apple 应用内购买和 App Store Connect 自动化管理。

| 包 | 职责 |
|---|---|
| `appstoreserver` | Apple App Store Server Library 的 Go 对应实现，提供 30 个 API 操作、鉴权客户端和签名数据验证 |
| `appstoreserver/model` | 服务端 API、通知、订阅挽留消息和 Advanced Commerce 的请求、响应模型与枚举 |
| `appstoreserver/offers` | 旧版促销优惠、促销优惠 V2 和首次订阅优惠资格签名 |
| `appstoreserver/advancedcommerce` | StoreKit Advanced Commerce 应用内请求签名 |
| `appstoreserver/receipt` | 从旧版 receipt 中提取交易 ID，辅助迁移 |
| `appstoreconnect` | 52 个强类型操作，覆盖应用、构建、上传、内购商品、订阅、本地化、可售范围和价格管理 |
| `appstoreconnect/model` | App Store Connect 的 JSON:API 请求和响应模型 |
| `signin` | Sign in with Apple 登录，提供支持请求上下文的接口 |

兼容基线为 [Apple Java Library 5.2.0](https://github.com/apple/app-store-server-library-java/tree/v5.2.0) 和 [App Store Connect OpenAPI 4.5.1](https://developer.apple.com/documentation/appstoreconnectapi)。
详细说明见[兼容与迁移文档](docs/compatibility.md)、[Connect 接口清单](docs/connect-api.md)和[可编译示例](examples/sdk/examples.go)。

## 安装

要求 **Go 1.26 或更高版本**，最低版本由当前 `golang.org/x/crypto` 的 OCSP 依赖决定。

```sh
go get github.com/sunbeam27/app-store@latest
```

`v0.2.0` 提供上述新模块，也可以指定版本安装：

```sh
go get github.com/sunbeam27/app-store@v0.2.0
```

旧版根包接口继续保留，使用说明见 [docs/legacy.md](docs/legacy.md)。

## App Store Server API

配置应用内购买签名密钥、Key ID、Issuer ID、Bundle ID 和运行环境：

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

客户端还支持订单查询、订阅状态筛选、交易和退款分页查询、通知查询、续期日期延长、消费信息 V1/V2、App Account Token 更新、应用交易查询和完成交易。订阅挽留消息操作使用同一个客户端。

API 调用返回 Apple 的原始响应模型，验签由独立的验证器显式执行。

### 验证交易和通知

从 [Apple PKI](https://www.apple.com/certificateauthority/) 下载可信的 Apple 根证书，传入 DER 或 PEM 格式的证书内容：

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

生产环境还需要设置 `AppAppleID: model.Ptr(yourAppAppleID)`。
验证器会检查证书链、Apple 签名证书扩展、JWS 签名，以及各类载荷对应的应用和环境标识。

验证接口覆盖交易、续期信息、通知、应用交易和订阅挽留实时请求。在线检查使用当前时间验证 OCSP 状态；已验证证书链最多缓存 15 分钟，并受证书到期时间限制。离线检查按载荷的签名时间验证证书有效性，与 Apple 官方库保持一致。Xcode 和 LocalTesting 必须显式配置，仍会校验应用和环境。

处理 HTTP 通知请求体时，先解析 `signedPayload` 字符串，再调用 `VerifyAndDecodeNotification`。内部的 `signedTransactionInfo` 和 `signedRenewalInfo` 是各自独立的 JWS，使用前应分别验签。

### 筛选交易历史

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

使用下一页 revision 时，保持筛选条件一致。省略版本参数时默认使用 V1，与 Java 重载接口一致；查询当前版本的交易历史时显式传入 V2。

## App Store Connect

为 Connect 客户端单独配置 App Store Connect 集成密钥：

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

个人密钥使用 `IndividualKey: true`，并省略 Issuer ID。两个 API 客户端都支持自定义 HTTP 客户端和外部令牌提供函数。

### 创建内购商品

```go
request := appstoreconnect.NewInAppPurchaseRequest(
    appID, "100 Coins", "com.example.coins100", connectmodel.InAppPurchaseTypeConsumable,
)
product, err := connect.CreateInAppPurchase(ctx, request)
```

该操作创建商品记录。后续通过对应接口配置本地化信息、可售范围和价格，提交审核需要单独操作。

### 创建自动续期订阅

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

自动续期订阅归属于订阅组，使用专门的接口管理。

### 上传 IPA 或 PKG

SDK 使用 Apple 的 [Build Uploads API](https://developer.apple.com/documentation/appstoreconnectapi/post-v1-builduploads)，依次创建上传记录、为各文件预留上传位置、流式上传指定字节区间，并提交文件。

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

通过 context 截止时间限制上传和处理等待时长。SDK 会根据 `.ipa`、`.pkg` 后缀推断 UTI，其他受支持的文件类型需要显式传入 UTI。校验和对象是可选项，可通过 `UploadFile.Checksums` 提供。

也可以分别调用 `UploadBuildFile`、`CreateBuildUploadFile`、`UploadOperations`、`CommitBuildUploadFile` 和状态查询接口。上传失败时不会自动提交文件；`UploadError` 保留上传和文件 ID，高层上传接口也会在后续步骤失败时返回已有上传记录。上传 URL 只接收 Apple 指定的操作请求头，不附加 Connect API 的 Bearer Token。SDK 不会自动重试写操作。

### 筛选和分页

GET 方法接收 `url.Values`，用于配置筛选条件、返回字段、关联资源和条数限制：

```go
apps, err := connect.ListApps(ctx, url.Values{
    "filter[bundleId]": {"com.example.app"},
    "limit":           {"50"},
})
```

将响应中的 `response.Links.Next` 传给 `Client.Do`，即可读取下一页。其他 JSON API 操作也可使用 `Do`；带鉴权的请求限定在客户端配置的 API 来源地址内。

## Sign in with Apple 登录

```go
import "github.com/sunbeam27/app-store/signin"

login := signin.NewAppleAuthClient(clientID, teamID, keyID, string(signInKeyPEM))
tokens, err := login.ExchangeCodeContext(ctx, authorizationCode, redirectURI)
claims, err := login.VerifyIDTokenWithNonceContext(ctx, idToken, expectedNonce)
```

登录包还提供支持 context 的令牌刷新和撤销接口，原有方法继续保留以兼容现有调用。

## 模型和错误处理

可选标量字段使用指针，区分字段缺失与零值、`false`。枚举类型保留未知的字符串或数值，模型通过 `UnknownFields` 保存未知 JSON 字段。JSON:API 的 `included` 联合类型使用 `json.RawMessage` 表示。

使用 `errors.As` 提取 `*appstoreserver.APIError` 或 `*appstoreconnect.APIError`，即可查看 HTTP 状态、响应头、重试时间和原始响应体。服务端错误保留 Apple 原始错误码，Connect 错误保留 JSON:API 错误数组及字段位置。验签错误提供 `VerificationStatus`，可区分允许重试的 OCSP 故障。

## 开发和发布

```sh
go test -race ./...
go vet ./...
go build ./...
```

模型可从锁定的上游输入重复生成，操作步骤见 [docs/compatibility.md](docs/compatibility.md)。测试覆盖 Apple 官方 JSON、签名数据、Xcode 和 receipt 样本，动态生成的证书与 OCSP 场景，以及完整的模拟分片上传流程。

SDK 以带版本的 Go module 和源码归档发布。在 `main` 上更新 `.github/release-version` 即可申请发布；工作流会先检查依赖、race、vet 和构建结果，再创建版本。已有标签和 Release 不会被覆盖。
