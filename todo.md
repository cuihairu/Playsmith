# TODO

> 代码类发现账本（文档审计波次 5，2026-11-05 记入；只动文档不改代码，代码问题在此登记）。

## 链码（chaincode/playsmith-assets）

- [x] 鉴权落地：`Mint` 校验 `role=MINTER` 证书属性；`Transfer` 校验调用方为 owner 或已授权 operator（代码内 TODO，`main.go:40` / `main.go:59`）
  - 证据：`chaincode/playsmith-assets/main.go:284`（`requireMinter`）、`:291`（`requireTransferAuth`）；`Mint` 调用于 `:31`、`Transfer` 于 `:62`；测试 `TestMintRequiresMinterRole`/`TestTransferOwnerAndOperator` 通过（`go test ./...`，2026-11-05）
- [x] `ListForSale` 校验 seller 余额 ≥ amount（SPEC.md Authorization 一节的 spec target）
  - 证据：`chaincode/playsmith-assets/main.go:80`（`requireBalance`）、`:277`；测试 `TestListForSaleRequiresBalance` 通过
- [x] `Trade` 事件补 `price`/`priceHash` 字段（SPEC.md 承诺携带；当前公共/私有上架均不带）
  - 证据：`chaincode/playsmith-assets/main.go:124-131`（公共带 `price`、私有带 `priceHash`）；测试 `TestTradeEventPriceFields`/`TestPrivateTradeEventPriceHash` 通过
- [x] `ask` 公共卖单值改为 JSON（含 `ts`），与私有卖单统一（当前为 `"<price>:<amount>"` 拼接串）
  - 证据：`chaincode/playsmith-assets/main.go:85`（JSON `{amount, price, ts}`）、`:241`/`:246`（私有含 `ts`）；测试 `TestPublicAskIsJSONWithTS` 通过

## API Gateway（cmd/api）

- [x] 修复 `GET /api/v1/assets/{id}/balance/{owner}` 路由解析：`main.go:179` 只在 `len(parts)==4` 时命中 `balance`，实际路径切分后为 3 段，永远 404
  - 证据：`cmd/api/main.go:233`（`len(parts)==3`）；测试 `TestBalanceRouteParses`/`TestBalanceRouteWrongSegments404` 通过
- [x] 接入 Fabric Gateway SDK，替换 `internal/sdk` 的 `NoopClient`（当前所有提交返回 `TX_NOOP`）
  - 证据：`internal/sdk/gateway.go`（`GatewayClient`+`NewGatewayClient`）、`cmd/api/main.go:176-190`（环境变量切换）；README 路线图勾选
- [x] JWT 鉴权与 gRPC 面（README 路线图项）
  - 证据：`internal/auth/jwt.go`（HS256 签发/校验，`JWT_SECRET`≥32B，iss=`playsmith`/aud=`game-server`）；`cmd/api/main.go:issueToken`（`X-Issue-Secret` 签发）、`authMiddleware`（REST bearer）、`grpcAuthUnaryInterceptor`（gRPC bearer）；`internal/gateway/service.go`（gRPC 面，镜像 REST）；proto `proto/playsmith/v1/gateway.proto`；测试 `TestIssueValidate`/`TestAuthMiddlewareRequiresToken`/`TestGRPCAuthInterceptor`/`TestMint` 等通过（`go test ./...`，2026-11-06）
- [x] `market/cancel` 请求体 struct 无 json tag（当前只接受 Go 默认大写键 `TokenID/Seller/ListingID`，与其余端点的小写键风格不一致）
  - 证据：`cmd/api/main.go:106-108`（`json:"tokenId"` 等小写 tag）；测试 `TestCancelUsesLowercaseJSON` 通过

## Indexer / 部署

- [x] `cmd/indexer` 真实现：订阅链码事件（TransferSingle/TransferBatch/AskCreated/AskCanceled/Trade）并 upsert Postgres（当前为 10s tick 占位）
  - 证据：`cmd/indexer/main.go:88`（`HandleEvent` 分发 5 类事件）、`:26-65`（schema 建表）、`:300`（`ChaincodeEvents` 订阅）；测试 `TestTransferSingleUpsert` 等通过
- [x] 补 `playsmithPvt` 私有数据集合的 collections 配置 JSON（`ListForSalePrivate`/`CancelListing` 已调用 `PutPrivateData`，无配置文件则部署失败）
  - 证据：`deploy/collections.json`（`playsmithPvt` 集合定义）
