# 兼容与迁移说明

新包以 Apple App Store Server Java Library **v5.2.0** 为基线，对应提交
`2d50adecdcf4c5cd8fbe285eaad305ef3ef60c17`；App Store Connect 部分实现官方 OpenAPI **4.5.1** 中选定的操作。
兼容目标是对齐传输契约以及文档规定的验签和签名行为，同时采用 Go 的命名、context 和错误返回方式。

## 包与接口对应关系

| Apple Java 组件 | Go 包或 API |
|---|---|
| AppStoreServerAPIClient | `appstoreserver.NewAppStoreServerAPIClient` |
| APIException / APIError | `appstoreserver.APIError` / `APIError*` 常量 |
| SignedDataVerifier | `appstoreserver.NewSignedDataVerifier` |
| VerificationException | `appstoreserver.VerificationError` |
| model | `appstoreserver/model` |
| PromotionalOfferSignatureCreator | `appstoreserver/offers` |
| PromotionalOfferV2SignatureCreator | `appstoreserver/offers` |
| IntroductoryOfferEligibilitySignatureCreator | `appstoreserver/offers` |
| AdvancedCommerceInAppSignatureCreator | `appstoreserver/advancedcommerce` |
| ReceiptUtility | `appstoreserver/receipt` |

服务端 SDK 提供 30 个 API 操作，通过可选的交易历史版本和图片尺寸参数对应 Java 的重载方法。HTTP 响应保留原始签名数据。
Connect 已支持的操作见[接口清单](connect-api.md)。Connect 作为独立 API 模块，拥有自己的客户端、鉴权配置、JSON:API 错误和资源模型。

## 面向 Go 的适配

- 公开标识符使用 `ID`、`URL`、`UUID`、`SKU`、`UTI` 等缩写形式。
- 网络调用和验签方法接收 `context.Context`。
- 模型使用结构体，可选标量字段使用指针，必填字段使用值类型。发送请求前应设置必要字段；Java 构造函数的校验没有被扩展为通用 Go 校验框架。
- 枚举以字符串或 `int32` 类型常量表示，解码时可保留新出现的枚举值。
- 未知对象字段在解码、编码之间得以保留。可选字段的 `null` 解码为 `nil`，再次编码时可能省略，与 Apple 模型中的可空值处理方式一致。
- UUID 字段使用字符串。调用方负责请求中的 UUID 格式，旧版优惠签名器的 nonce 参数会额外校验 UUID 格式。
- Xcode 的小数时间戳精确地向零截断，负时间戳也采用相同规则。
- 每个验签器实例独立配置根证书和应用信息，不依赖全局可变的信任存储。
- 在线 OCSP 检查无法确认可信状态时拒绝验证。成功的证书链最多缓存 15 分钟，并受证书有效期限制。委托响应者必须显式拥有 OCSP 签名用途。
- 上传通过 `io.ReaderAt` 流式读取数据，失败时保留上传记录 ID，将文件提交和异步处理作为独立步骤。

## 从 v0.1 迁移

根包和原有公开方法签名继续保留。Sign in with Apple 的实现迁入 `signin`，根包通过类型别名和转发函数兼容现有导入。
当前 OCSP 依赖要求最低使用 **Go 1.26**。

| 旧版用法 | 新版用法 |
|---|---|
| ParsePrivateKey + NewToken + NewClient | NewAppStoreServerAPIClient(ClientConfig) |
| WithSandbox() | ClientConfig.Environment = model.EnvironmentSandbox |
| GetTransactionInfo 返回解码后的载荷 | GetTransactionInfo 返回 TransactionInfoResponse，再显式验证其中的 JWS |
| DecodeClaims / DecodeNotificationV2 | SignedDataVerifier.VerifyAndDecode* |
| ParseNotificationRequest(body) | 解析 signedPayload 包装，再调用 VerifyAndDecodeNotification(ctx, signedPayload) |
| OrderLookupResponse.DecodeTransaction | 用已配置的验签器逐一验证原始 SignedTransactions JWS |
| 根包中的 AppleAuthClient | signin.AppleAuthClient；需要请求取消能力时使用 *Context 方法 |

旧版根包验证器保留原有的证书和签名检查行为，由调用方自行校验应用身份。新接入应使用已配置的验证器校验应用和环境。
处理通知时，还应分别验证内部交易和续期信息的 JWS，与 Apple 官方库的使用方式一致。

## 重新生成 SDK

获取锁定的上游源码后，执行：

```sh
git clone --branch v5.2.0 --depth 1 https://github.com/apple/app-store-server-library-java.git /tmp/apple-java
python3 scripts/generate_models.py server /tmp/apple-java
python3 scripts/generate_server_api.py /tmp/apple-java
python3 scripts/generate_models.py connect scripts/upstream/connect-4.5.1.json
python3 scripts/generate_connect_api.py scripts/upstream/connect-4.5.1.json
```

生成器会核对 Java 提交和 Connect 规范版本，拒绝使用其他版本。所需的官方 OpenAPI 子集保存在 `scripts/upstream/connect-4.5.1.json`，即使 Apple 更新当前下载文件，也能离线重现 Connect 代码。
升级前应先审查契约变化，再有意更新源码快照和生成器基线。生成器自身不会发起网络请求。

生成文件保留来源说明，Apple 许可原文见 `LICENSE.apple.txt`。官方测试样本保存在 `appstoreserver/testdata/apple`，升级兼容基线时应同步更新相关测试。

CI 使用 Go 1.26 和当前稳定版验证，不需要 Apple 凭据，也不会修改 App Store Connect 账号数据。真实账号的权限、商品审核要求和实际二进制处理结果，仍需由接入应用完成集成验证。
