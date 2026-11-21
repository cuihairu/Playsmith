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

## 审核发现

- [x] 审核发现：JWT 的 subject、MSP 和 role 未绑定到 Fabric 身份或按操作授权，所有持有效令牌的调用均使用同一配置证书，导致不同游戏服可共享其链上权限且无法按自身身份交易（cmd/api/main.go:353）。
  - 证据（按操作授权臂已修）：`internal/auth/authorize.go` `Claims.Authorize`（所有已鉴权调用要求非空 mspId+role；`Mint`/`BatchMint` 要求 `role=MINTER`，对齐 SPEC Authorization 与链码 `requireMinter`）；`cmd/api/main.go` REST `authMiddleware` 与 gRPC `grpcAuthUnaryInterceptor` 均调用 `Authorize`（经 `gatewayOperation`/`operationFromFullMethod` 映射操作）；测试 `TestAuthMiddlewareDeniesNonMinterMint`（PLAYER→Mint→403）、`TestAuthMiddlewareAllowsMinterMint`（MINTER→Mint→200）、`TestAuthMiddlewareRequiresIdentity`（空 mspId/role→403）、`TestGRPCAuthInterceptorDeniesNonMinterMint`（PLAYER→Mint→PermissionDenied）、`TestGRPCAuthInterceptorQueryNeedsIdentity`、`TestAuthorizeMinterGate` 等通过（`go test ./...`，2026-11-09）。
  - 绑定每游戏服 Fabric 身份臂仍 BLOCKED：需真实机器/凭据（Fabric CA 为每游戏服颁发 X.509 证书 + 私钥托管），无法在仓内验证，未修。本行 `[x]` 仅覆盖按操作授权臂；身份隔离臂单独重登为 `- [Critical-BLOCKED] JWT 身份隔离仍未闭环`（现 `todo.md:75`，2026-11-13 codex 审），不在本行口径内。按操作授权臂的证据 SHA：`5f0c8be`（`Claims.Authorize` 门禁）；链码侧对应的角色/卖家/owner 门禁在本轮 `e922b90` 补齐。
- [x] 审核发现：HTTP 和 gRPC 均监听明文连接，直接部署时签发密钥及 bearer token 会在网络中明文传输，泄露后可伪造令牌或冒用调用者（cmd/api/main.go:336）。
  - 证据：`cmd/api/main.go` `serveGRPC` 在设 `TLS_CERT_PATH`/`TLS_KEY_PATH` 时用 `credentials.NewServerTLSFromFile` 加 `grpc.Creds`；HTTP 用 `http.ListenAndServeTLS`；两者都未配置时回退明文（开发模式）；**只配置其中一个会启动失败**（`tlsConfig()`，`cmd/api/main.go:481`；commit `f766404`，测试 `TestTLSConfigRejectsHalfConfiguration`）；`tlsPaths()` 读环境变量；测试 `TestTLSPaths` 通过。
- [x] 审核发现：所有 gRPC 交易响应只填充 result 而从未填充 proto 定义的 tx_id，真实链码无返回值的交易也无法提供交易标识供客户端追踪（internal/gateway/service.go:45）。
  - 证据：`internal/sdk` `Client.Submit`/`SubmitWithTransient` 返回 `(result, txID, error)`；`GatewayClient` 用 `SubmitAsync`+`commit.Status()` 取 `TransactionID`；`internal/gateway/service.go` `txResponse` 填充 `TxId`（含无返回值交易如 `CancelListing`）；测试 `TestMint`（断言 `TxId`）/`TestCancelListingCarriesTxID` 通过。

## 源码分层分析 2026-11-07

### a) 抽象层清单

- 接入层：装配 Fabric 后端、JWT/TLS、REST 与可选 gRPC 生命周期；代表 `cmd/api/main.go:226`。
- 认证层：签发/校验平台托管 JWT 声明；代表 `internal/auth/jwt.go:20`。
- Fabric 适配层：抽象提交/查询并把真实 Gateway 与 Noop 开发实现隔开；代表 `internal/sdk/fabric.go:7`。
- 账本协议层：执行 ERC-1155 风格资产、市场、授权与事件协议；代表 `chaincode/playsmith-assets/main.go:14`。
- 事件投影层：把链码 JSON 事件事务化派生为 Postgres 查询模型；代表 `cmd/indexer/main.go:85`。
- 部署契约层：定义私有数据集合与成员/背书策略；代表 `deploy/collections.json:1`。

### b) 超大文件候选（>1000 行）

- [x] LAY-001 [P1] 拆分 `internal/gatewaypb/gateway.pb.go`（1172 行）——生成物与源码提交混在同一目录，升级/维护时容易掩盖手写接口变更（`internal/gatewaypb/gateway.pb.go:1`）。
  - 证据：生成物迁至独立生成目录 `internal/gatewaypb/gen/`（`gen/gateway.pb.go`、`gen/gateway_grpc.pb.go`，均带 `DO NOT EDIT` 头，git 识别为 rename）；`Makefile` 提供 `proto`（再生成）与 `proto-check`（禁改守卫：生成物漂移即 `ERROR: ... out of date` 失败，负测试手工篡改 `gateway.pb.go` 确触发，改回后通过）；`gen/doc.go` 包注释 + `gen/README.md` 标记；proto `go_package` 更新为 `playsmith/internal/gatewaypb/gen;gatewaypb`；`make proto-check` / `go build ./...` / `go vet ./...` / `go test ./...` 全部通过（2026-11-09）。

### c) 目录/分类不合理处

- `internal/gatewaypb/gen/gateway.pb.go:1` 和 `internal/gatewaypb/gen/gateway_grpc.pb.go:1` 均为 protobuf 生成文件；已按建议用生成规则目录 `gen/` 与构建禁改标记（`make proto` / `make proto-check`）隔离，避免误作手写内部模块维护（LAY-001 已解决）。
- `internal/sdk/fabric.go:1` 与 `internal/sdk/gateway.go:1` 以文件名区分客户端抽象与实现，但同包内语义分组不明显；后续引入多后端时应将接口/错误与各实现分成稳定包边界。

- [x] 审核发现：`build`、`test` 均未依赖 `proto-check` 且仓库没有自动执行该检查的配置，生成文件被手改或 proto 未重新生成时正常构建仍会成功，禁改守卫无法生效（Makefile:23）。
  - 证据：`Makefile:29`/`Makefile:32`（`build: proto-check`、`test: proto-check`，守卫先于编译/测试）；负测试：手工篡改 `gen/gateway.pb.go` 后 `make test` 在守卫阶段即报 `ERROR: gateway.pb.go is out of date` 并失败（退出码 2），还原后通过；`make build` 同样先跑守卫；`go build ./...`/`go vet ./...`/`make proto-check`/`go test ./...` 全绿（commit `2e92e6c`，2026-11-09）。
- [x] 审核发现：生成检查逐字比较含 protoc 版本号的生成文件，但 `proto-install` 只固定插件版本而未安装或校验 protoc 版本，使用其他版本 protoc 时即使协议未变也会错误报生成物过期（Makefile:40）。
  - 证据：`Makefile:22`（`PROTOC_VERSION := 3.21.12` 固定）、`Makefile:39-49`（`proto-doctor` 校验 protoc 存在且版本精确匹配）；`proto`/`proto-check`/`proto-install` 均先过 `proto-doctor`；负测试：`make proto-check PROTOC_VERSION=9.9.9` 报 `ERROR: protoc version mismatch: found 3.21.12, need 9.9.9`（不再误报"生成物过期"）、`PROTOC=/nonexistent` 报 not found、`make proto PROTOC_VERSION=9.9.9` 拒绝再生成；全绿（commit `2e92e6c`，2026-11-09）。
- [ ] 审核发现：`Authorize` 仅校验可签发的 JWT 角色并用共享 Fabric MINTER 证书提交 Mint/BatchMint，未把 JWT subject/mspId 绑定到独立链上身份，身份越权风险未闭环（internal/auth/authorize.go:26）。
  - BLOCKED 缺什么：与第 76 行 Codex `[Critical-BLOCKED]` 为同一问题（先后两次登记）。需 (a) Fabric CA 为每个游戏服单独签发 X.509 并下发公钥；(b) 私钥托管/轮换机制（Vault/KMS），按 subject/mspId 选择签名身份；(c) 多 MSP 真实测试环境覆盖跨身份隔离用例。三者均不在仓内可修范围，详见第 76 行，不重复展开。
  - 仓库内已落地的部分（见第 78 行）：`JWT_IDENTITY_MAP` 绑定门禁（commit `a658533`、`1ffd41c`）已决定「哪些主体有资格行动/可被签发令牌」，但从不把 subject/mspId 映射到独立链上身份——进程内仍只有一个签名证书。故本项仍不勾选。

## Codex 深审发现（2026-11-13，luna profile 近5 commit 审核；已修 13/14，余 1 条 BLOCKED 见下）

- [x] [Critical] Buy 不限制订单成交数量 | 位置 [chaincode/playsmith-assets/main.go:107](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:107) | 问题：解析了 `ask.Amount`，却直接用请求的 `amount` 扣卖家余额，之后无条件 `DelState(askKey)`。历史残留 | 影响：卖单数量为 1、卖家余额为 100 时，请求购买 100 仍可成功；少量购买也会删除整个剩余订单。公开、私有及旧格式订单均受影响 | 修复方向：明确整单或部分成交语义，校验数量并正确保留余量；补超额及部分成交测试。
  - 证据：Buy 先校验 `amount > ask.Amount → "amount exceeds listing"` 并整体回滚；成交等于挂单量时删单，小于挂单量时按 `{amount, price|priceHash, ts}` 回写余量。测试 `TestBuyRejectsExcessAmount`（超额被拒、卖家余额与挂单原样保留）、`TestBuyPartialFillRetainsRemainder`（部分成交后余量挂单仍在、续买清单）、`TestBuyFromSelf`。commit `e922b90`。
- [Critical-BLOCKED] JWT 身份隔离仍未闭环 |位置 [internal/auth/authorize.go:26](/home/cui/workspaces/Playsmith/internal/auth/authorize.go:26)、[cmd/api/main.go:96](/home/cui/workspaces/Playsmith/cmd/api/main.go:96) | 问题：`Authorize` 只要求非空 MSP/role，并限制铸造角色；通过后直接调用共享的 `s.Fabric.Submit(...)`，未传递或使用 subject/MSP。`5f0c8be` 修复了 MINTER 门禁，但未解决身份共享 | 影响：任意有效 PLAYER 令牌仍可转移共享证书拥有或获授权的资产；不同游戏服无法隔离权限。新增测试仅用 fake client，未覆盖此路径 | 修复方向：根据可信身份映射选择 Fabric 签名身份，或实现链上可验证的委托授权；补跨 subject/MSP 的隔离测试。
  - BLOCKED 缺什么：仓库内无凭据可验证。需要 (a) Fabric CA 为每个游戏服单独签发 X.509 证书并下发公钥；(b) 私钥托管/轮换机制（Vault/KMS），链上身份按 subject/mspId 选择签名身份；(c) 多 MSP 的真实测试环境以覆盖跨身份隔离用例。缺 (a) 就无法签发第二身份，缺 (b) 则无法证明私钥不外泄，缺 (c) 就无法写「跨 subject/MSP 的隔离测试」。三者都不在代码库范围内，故本轮不改代码、不勾选。
  - 仓库内骨架已落地（commit `a658533`，文档侧 `df65f8e`；位置指针更新为 `cmd/api/main.go:96`）：新增 `internal/auth/identity.go` 的「可信主体映射」——配置 `JWT_IDENTITY_MAP="subject:mspId[:role];..."` 后，校验通过的 claims 必须命中其中一条绑定（subject、mspId，以及绑定中钉住的 role），否则在 REST 中间件与 gRPC 拦截器里先于任何 Fabric 调用被 403 / `PermissionDenied` 拒绝；未设置时保持原行为（不校验主体绑定），取值格式非法则启动即失败（与「TLS 只配一半即拒绝启动」同一口径）。
    - 证据：`internal/auth/identity_test.go` 5 例（空/多绑定解析、6 种畸形格式全部拒绝、绑定命中、角色钉住命中、角色钉住冲突、未绑定主体、绑错 mspId、nil 注册表放行）；`cmd/api/main_test.go` 4 例（REST 未绑定主体 403 且未触达 fake client、绑定主体 200、角色钉住冲突 403 且未触达 fake、gRPC 未绑定主体 PermissionDenied）。`make build` / `make vet` / `make test` 双模块绿；`pnpm --filter @playsmith/docs test` 11/11 绿、`pnpm --filter @playsmith/docs build` 绿。文档：`docs/configuration.md` 与其 zh 镜像各补 `JWT_IDENTITY_MAP` 行（保持「代码读到的 env ↔ 文档列出」双向一致），`docs/architecture.md` §2 改为如实描述「映射只决定谁能行动，从不挑选证书」。
    - 仍 BLOCKED：该映射只决定「谁能行动」，从不挑选签名证书——进程内仍只有一个 `GatewayClient`（`cmd/api/main.go:243`），签名身份选择臂仍缺 (a)(b)(c)，本项不勾选。
  - 补全签发侧（commit `1ffd41c`）：设置了 `JWT_IDENTITY_MAP` 时，`POST /api/v1/auth/token` 同样拒绝未绑定主体（400）——未绑定主体不再可能被签发出令牌、要到请求侧才被 403，配置错字在签发那一刻就暴露。测试 `TestIssueTokenRejectsUnboundSubject`（400）、`TestIssueTokenAllowsBoundSubject`（200 且令牌非空）；nil 注册表（未设映射）由既有 `TestIssueTokenEndpoint` 覆盖。门禁：`make test` 双模块绿、`pnpm --filter @playsmith/docs test` 11/11 绿、`build` 绿；文档 `configuration.md` 与其 zh 镜像的 `JWT_IDENTITY_MAP` 行补签发侧口径与 `cmd/api/main.go:329` 引用。
- [x] [Critical] 任意调用方可替资产所有者授权 | 位置 [chaincode/playsmith-assets/main.go:192](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:192) | 问题：`SetApprovalForAll` 仅检查参数非空，随后直接 `PutState(key, []byte(v))`；没有验证调用者是否为 owner。历史残留 | 影响：攻击者可先将自己或共享网关证书设为任意 owner 的 operator，再调用 Transfer 盗取资产；已有转账鉴权因此可被绕过 | 修复方向：只有经验证的 owner 才能修改授权；补第三方自授权失败及状态不变测试。
  - 证据：'SetApprovalForAll' 增加 `invokerIdentity(ctx) != owner → "not authorized: only owner can set approval"`。测试 `TestSetApprovalForAllIsOwnerOnly`：第三方自授权失败且 `IsApprovedForAll` 仍为 false（未写入状态），owner 授权生效。commit `e922b90`。
- [x] [Critical] 余额和供应量存在 uint64 溢出 | 位置 [chaincode/playsmith-assets/main.go:342](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:342)、[main.go:371](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:371) | 问题：两处均直接执行 `strconv.FormatUint(n+amt, 10)`。历史同型残留 | 影响：余额或供应量达到 `MaxUint64` 后再加 1 会写成 0；合法角色门禁不能保护账本数值完整性 | 修复方向：加法前检查 `amt > MaxUint64-n`，失败时拒绝整个交易；补余额与供应量边界测试。
  - 证据：新增 `checkAdd`（`amt > MaxUint64-n → "numeric overflow"`），`putAddBalance` 与 `addSupply` 均先过它再写。测试 `TestBalanceOverflowIsRejected`、`TestSupplyOverflowIsRejected`：触顶后拒绝该交易且余额/供应量不卷绕为 0。commit `e922b90`。
- [x] [Critical] 未经卖家授权即可上架、撤单并串联盗取资产 | 位置 [chaincode/playsmith-assets/main.go:77](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:77)、[main.go:240](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:240)、[main.go:266](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:266) | 问题：公开/私有上架只执行 `requireBalance(...)`，撤单直接 `DelState(askKey)`，均不校验 seller 身份。历史同型残留 | 影响：任意调用者可冒充持币卖家创建订单，再通过 Buy 将资产转给自己；也可撤销他人订单 | 修复方向：在三个入口验证卖家或其合法代理权限；补不同身份之间的拒绝测试。
  - 证据：'ListForSale'/'ListForSalePrivate'/'CancelListing' 均加 `requireTransferAuth(ctx, seller)`（卖家本人或已授权 operator）；`CancelListing` 另在 `DelState` 前校验挂单存在（`DelState` 对不存在的键也成功，原代码会凭空发 `AskCanceled`）。测试 `TestListForSaleRejectsForeignIdentity`（第三方上架/撤单被拒、owner 成功、撤单后键消失、未知 listingId 报 `listing not found`）、`TestListForSalePrivateRejectsForeignIdentity`。commit `e922b90`。
- [x] [Critical] 自转账会增发余额，批量交易会覆盖前序写入 | 位置 [chaincode/playsmith-assets/main.go:66](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:66)、[main.go:172](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:172) | 问题：先 `putSubBalance` 再 `putAddBalance`，批量操作又循环调用单笔方法；所用 Fabric shim 的 `GetState` 不读取本交易尚未提交的写集。历史残留 | 影响：余额 100 自转 10，最终写入 110；Buy 的 seller=buyer 同样受影响。批量重复 token/owner 会丢失前序增减，同 token 向不同接收者铸造也会算错供应量。MockStub 立即更新 State，掩盖了真实语义差异 | 修复方向：自转账保持余额不变；批量先聚合变化，再逐键读取、校验和写入一次；使用模拟真实写集语义的测试。
  - 证据：'Transfer' 在 `from == to` 时跳过两次写（保持余额不变）；`BatchMint`/`BatchTransfer` 不再循环调用单笔方法，改为 `pendingBalance` 按状态键聚合 adds/subs，每键只读一次、校验一次、写一次，与真实 Fabric 写集语义一致。测试 `TestSelfTransferKeepsBalance`、`TestBatchMintAggregatesDuplicateKeys`（同 token+接收者 7+8=15、供应量 19）、`TestBatchTransferAggregatesDuplicateKeys`（含批量自转不改变余额）。commit `e922b90`。
- [x] [Major] BatchMint 事件丢失真实接收者 | 位置 [chaincode/playsmith-assets/main.go:175](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:175)、[cmd/indexer/main.go:179](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:179) | 问题：最终事件写为 `"to": "batch"`，Indexer 将其作为所有资产的实际 owner；真实 Fabric 每交易只保留一个事件，循环内的 TransferSingle 会被覆盖 | 影响：批量铸造被索引到虚构的 batch 账户。`TestTransferBatch` 恰好使用 `"to":"batch"` 且仅检查执行次数，未校验真实业务结果 | 修复方向：批量铸造事件携带逐项接收者，并同步修改消费协议和断言。
  - 证据：'BatchMint' 事件改为 `{operator, from, tos, ids, values}`（`tos` 与 `ids` 平行；真实 Fabric 每交易只保留最后一个事件，逐项发事件必然全丢）；`cmd/indexer` 的 `handleTransferBatch` 按 `tos[i]` 逐项入账，无 `tos` 时回退 `to`。测试 `TestBatchMintAggregatesDuplicateKeys`（链码侧断言 `tos` 且无旧 `to` 字段）、`TestTransferBatchCarriesRecipients`（indexer 侧 alice/bob 各得 1/2）。commit `e922b90`、`db27d09`。
- [x] [Major] Indexer 数值链路丢精度且不能承载 uint64 | 位置 [cmd/indexer/main.go:124](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:124)、[main.go:155](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:155) | 问题：事件解码为 `map[string]any`，数量和价格经 `float64` 再转 `int64`，数据库使用 BIGINT；上架、成交及批量路径存在同型转换 | 影响：`9007199254740993` 会被舍入为 `9007199254740992`，超过 int64 范围的合法链码数值也无法正确保存 | 修复方向：采用精确数字解码和范围校验，数据库使用能完整表达 uint64 的数值类型；补大数测试。
  - 证据：'decodePayload' 改用 `json.NewDecoder(...).UseNumber()`，`uintField` 走 `strconv.ParseUint`（范围外拒绝）；`balances`/`listings`/`trades` 数量列改 `NUMERIC`，参数以十进制文本传入并 `::numeric` 显式转换。测试 `TestLargeAmountKeepsEveryDigit`（`9007199254740993` 不被舍入）、`TestAmountOutOfRangeIsRejected`。commit `db27d09`。
- [x] [Major] Indexer 的扣款 SQL 引用了不存在的列 | 位置 [cmd/indexer/main.go:160](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:160)、[main.go:231](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:231) | 问题：两处更新包含 `updated_at=now()`，但 `balances` 建表只有 `token_id`、`owner`、`balance`。历史残留，`todo.md:29` 已勾选实现完成 | 影响：按仓库 schema 创建的新数据库无法投影普通转账和成交，事务会回滚；mockExec 总返回成功，现有测试不能发现 SQL 错误 | 修复方向：统一 schema 与 SQL，并提供已有数据库迁移及真实数据库回归测试。
  - 证据：'balances' 建表补 `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`，并加启动迁移 `ALTER TABLE balances ADD COLUMN IF NOT EXISTS updated_at`（覆盖已建库）；`ON CONFLICT` 分支也刷 `updated_at`。测试 `TestSchemaAndSQLAgree`、`TestBalancesUpsertRefreshesUpdatedAt`。commit `db27d09`。
- [x] [Major] TLS 配置不完整时降级为明文 | 位置 [cmd/api/main.go:269](/home/cui/workspaces/Playsmith/cmd/api/main.go:269)、[main.go:390](/home/cui/workspaces/Playsmith/cmd/api/main.go:390) | 问题：HTTP/gRPC 都只有在 `certFile != "" && keyFile != ""` 时启用 TLS，任一配置遗漏即走明文；`TestTLSPaths` 只验证环境变量读取 | 影响：原本意图启用 TLS 的部署可能暴露 bearer token 及签发密钥；`todo.md:39` 的完成勾选未覆盖此异常路径 | 修复方向：证书和私钥只配置一个时拒绝启动；补两种缺失配置测试。
  - 证据：新增 `tlsConfig()`（`cmd/api/main.go:481`）：只设一半即返回错误，REST 侧 `log.Fatal`、gRPC 侧由 `serveGRPC` 返回错误，都不再静默明文。测试 `TestTLSConfigRejectsHalfConfiguration`（未设/单设 cert/单设 key/双设四态）。commit `f766404`。
- [x] [Major] 事件处理失败和重启会永久漏账 | 位置 [cmd/indexer/main.go:280](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:280)、[main.go:340](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:340) | 问题：处理错误仅 `log.Printf(...)` 后消费下一条；订阅 `ChaincodeEvents(ctx, chaincodeName)` 未指定起点或 checkpoint，所用 SDK 默认从下一次提交开始 | 影响：暂时性数据库错误直接丢事件，初次启动及停机期间的历史交易不会补齐；流关闭后进程也直接退出 | 修复方向：持久化成功消费位置、恢复订阅，并对失败事件重试，禁止跳过后推进位置。
  - 证据：断点与事件投影在同一事务内写入 `indexer_state`（失败一起回滚）；启动 `resumeBlock` 读回并以 `client.WithStartBlock` 续订；处理失败经 `retry`（`retryMax = 3`）重试同一事件而非跳过后丢账；事件流关闭后按最后提交块重订阅。测试 `TestResumeBlockFromCheckpoint`（有值 42 / 无值 0 两态）。commit `db27d09`。
- [x] [Major] 依赖升级未完成 genproto 拆包迁移 | 位置 [chaincode/playsmith-assets/go.mod:28](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/go.mod:28) | 问题：`34658f3` 将 gRPC 升至 `v1.85.0`，却保留包含 `googleapis/rpc/status` 的旧整体模块 `genproto@…20230110…`；新版 gRPC 又依赖独立的 `genproto/googleapis/rpc@…20260817…`。只读 `go mod graph` 确认两者同时进入依赖图 | 影响：同一导入路径存在双重提供者，依赖补齐后会形成 ambiguous import，不能据版本号升级认定链码构建已验证 | 修复方向：升级整体 genproto 至拆包后的兼容版本，整理模块及校验和，并单独构建、测试链码模块。
  - 证据：'chaincode/playsmith-assets/go.mod' 的单体 genproto 升级至 `v0.0.0-20261104224025-d1e00e9f33ab`（已过拆包点，不再提供 `googleapis/rpc/status`），`go mod tidy` 后仅保留 `genproto/googleapis/rpc`。核对：`go list -deps ./...` 中 `googleapis/rpc/status` 只出现一次；直接 import 该包的探针可编译。commit `12b0228`。
- [x] [Major] 原始事件去重未阻止重复余额变更 | 位置 [cmd/indexer/main.go:129](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:129) | 问题：`INSERT ... ON CONFLICT DO NOTHING` 后未检查插入是否成功，仍继续执行 `handleTransfer` 或 `handleTrade` | 影响：重放同一事件时，原始记录只有一份，余额却重复扣增；成交记录的去重也无法撤销已经执行的余额修改 | 修复方向：仅在首次插入事件时执行投影，并在同一事务内完成；补重复投递测试。
  - 证据：'route' 取 `CommandTag`，`tag.RowsAffected() == 0` 时直接返回，不执行任何投影；断点写入随之跳过。测试 `TestDuplicateEventIsNotProjectedTwice`：重复事件只产生 1 条原始表写入，余额不再二次增减。commit `db27d09`。
- [x] [Major] 标准测试入口遗漏整个链码模块 | 位置 [Makefile:32](/home/cui/workspaces/Playsmith/Makefile:32) | 问题：`test` 只执行根模块的 `go test ./...`，不会进入具有独立 go.mod 的 `chaincode/playsmith-assets`；build、vet 同样遗漏 | 影响：`04f4fad`/`2e92e6c` 引入的标准检查入口无法验证核心链码，也无法发现 `34658f3` 对链码依赖造成的回归 | 修复方向：三个入口显式检查两个 Go 模块。
  - 证据：'build'/'test'/'vet' 各追加 `cd $(CHAINCODE_DIR) && $(GO) ...`。实测 `make test` 输出含 `cd chaincode/playsmith-assets && go test ./...` 且该模块 `ok`（`make build` 同样双模块编译通过）。commit `ef03c5a`。

## G2 文档审计待办（2026-11-14）

- [x] P1：建最小 vitepress 文档站（locales：`/`=en、`/zh/`=zh）+ README 双语对（README.md / README.zh.md）。
  - 证据：`docs/.vitepress/config.mts`（base `/Playsmith/`，root=en-US + zh=zh-CN，srcExclude 审计台账，`ignoreDeadLinks` 仅豁免跨树 `../` 证据链接）；英文页 `docs/index.md`/`features.md`/`configuration.md`/`build.md` + 中文镜像 `docs/zh/*`（features 路由表取自 `cmd/api/main.go:282-296`，配置表含全部 env 读取行号，构建表照 Makefile 实测）；部署 `.github/workflows/pages.yml`（pnpm + configure-pages + upload-pages-artifact `docs/.vitepress/dist`）；`pnpm-workspace.yaml`（`allowBuilds: {esbuild: false}`，pnpm≥11 语义，postinstall 缺席不影响构建）；`docs/package.json`（`@playsmith/docs`）；README.md 拆为英文 + README.zh.md（互链）。本地门禁：`pnpm install` 退出 0；`pnpm --filter @playsmith/docs build` 绿（client+server bundles、12 页含 zh/ 4 页、`/`↔`/zh/` 切换双向可达、`<html lang>` en-US/zh-CN 正确）；`make test`（proto 守卫 + go test）绿。线上探针旧值 404 属部署前快照，Pages 首次部署后需复测（见审计报告 G2 面 2）。Pages 首次部署后线上复测（2026-11-21，`https://cuihairu.github.io/Playsmith`）：`/zh/`、`/features`、`/zh/features`、`/configuration`、`/zh/configuration`、`/build`、`/zh/build` 均 200，`<html lang>` 分别为 zh-CN / en-US 正确；`/` 为 301 规范跳转至 `/Playsmith/`，跟随后 200；`/`↔`/zh/` 双向可达。
- [x] P2：文档站首访按 `navigator.language` 自动跳转 `/zh/`（增强项，基线为手动切换+显式链接；等用户拍板范围，不随建站默认实施）。当前站内切换仅手动：locale 菜单落 `/` 与 `/zh/`，无 rootLocale 重定向、无 navigator.language 探测。fleet-docs-audit-G2 2026-11-14。（2026-11-21 用户令「继续清任务」，视为对本增强项的拍板，落地实施。）
  - 证据：`docs/.vitepress/redirect.mjs` `redirectSnippet(base)` 产出内联脚本（`navigator.language` 以 `zh` 开头且当前页不在 `/zh/` 下时 `location.replace` 到同路径 `/zh/` 孪生页，保留 query/hash；真实资源路径与 base 外路径不动；`localStorage['ps-locale']` 一次性标记使自动跳转只在首访发生，之后手动切换是唯一语言来源——自动跳转不再与用户后续的手动选择打架）；`docs/.vitepress/config.mts` 经 `head` 注入，base 以同一常量插值不漂移；`docs/.vitepress/redirect.test.mjs` 以 node:vm 跑产物字符串本身 11 例（zh/zh-TW 首访跳转、`/features` 与 `/features.html` 两种可达形态都跳、已在 zh 不跳、en 不跳、二访不跳且标记不再覆写、8 类资源路径不跳、`404.html` 不跳、根页跳转、query+hash 保留、base 外不跳、隐私模式无 localStorage 不抛不跳）。门禁：`pnpm --filter @playsmith/docs test` 11/11 绿、`pnpm --filter @playsmith/docs build` 绿且 12 个产物 HTML 全含该脚本（对构建压缩后的产物原文再跑 7 例沙箱验证均正确）、pages workflow 构建前先跑 test、`make test` 双模块绿。commit `c860313`。
    - 修正扁平产物缺口（commit `07c5f37`）：本构建产物是扁平 `features.html`，`/features/` 在 GitHub Pages 上 404，可达形态只有 `/features` 与 `/features.html`；原「跳过一切带扩展名路径」的守卫会把页面一并拦下，导致 zh 用户落在 `/features.html` 不跳转。改为只跳过真实资源类型（css/js/mjs/map/png/svg/ico/webp/jpg/jpeg/gif/xml/txt/json/woff/woff2），并额外跳过 `404.html`，避免把回退页踢成第二次 404。
    - 线上复核（2026-11-21）：pages 运行 `completed success` 后，`https://cuihairu.github.io/Playsmith/` 与 `/zh/` 的已部署 HTML 均含该脚本（各 1 处）。`07c5f37` 修复部署后复测：`/`、`/zh/`、`/features.html`、`/404.html` 的已部署 HTML 均含新的 `404.html` 守卫，扁平产物修正已在线上生效。
    - 运维备忘（2026-11-21，踩坑后记录）：本 pages 工作流**不要用 `gh run rerun --failed`** 修 `deploy-pages` 失败——它会重跑 `upload-pages-artifact`，同一 run 内产生 2 个同名 `github-pages` 产物，`deploy-pages` 随即报 `Multiple artifacts named "github-pages"`。正确做法：`gh workflow run pages --ref main` 触发全新 run。`deploy-pages` 报 `Found 0 artifact(s)` + `Fetching artifact metadata failed` 属平台侧抖动，test/build 已绿时无需改代码。
    - 运维备忘（2026-11-21，踩坑后记录）：在线上探针里核对重定向脚本，标记**必须**用压缩后的真实形式 `+"404.html"`。若写成 `404.html";`（带分号），grep 会稳定返回 0 而产生假阴性——压缩产物里该守卫是 `+"404.html")` 而非 `;"`。假阴性一旦当真，会被误读成「脚本丢失/构建回归」。
    - 线上复核（2026-11-21，`ab8b592` 推送前，deploy 自 `35898d6`）：`ci` 与 `pages` 两个 workflow 在 `35898d6` 上均 `completed/success`。`/Playsmith/configuration` 与 `/Playsmith/zh/configuration` 的已部署 HTML 各含 `JWT_IDENTITY_MAP` 2 处（env 表行 + 注意点段落），签发侧门禁的文档口径已在线上；`/`、`/zh/`、`/features.html`、`/404.html` 各含 `+"404.html"` 守卫 1 处，首访重定向无回归。

## 代码卫生（2026-11-21 巡检，全仓 `TODO`/`FIXME` 清扫）

- [x] 清理 `internal/sdk/fabric.go` 过时的 `// TODO: implement with fabric-gateway`：该 TODO 暗示 Gateway SDK 接入尚未实现，但真实实现早已在 `internal/sdk/gateway.go`（`GatewayClient`/`NewGatewayClient`，对应本文件第 20 行「接入 Fabric Gateway SDK」勾选项）；`NoopClient` 只是 `FABRIC_GATEWAY_ENDPOINT` 未设时的开发兜底（`cmd/api/main.go:244-245`），并非未完成项。已改为准确注释，说明其为有意的开发占位并指向 `gateway.go`。
  - 证据：`internal/sdk/fabric.go:13-17`（新注释，删除误导 TODO）；`grep -rn "NoopClient" --include=*.go` 仅剩定义处与 `cmd/api/main.go:244` 兜底接入，无任何测试依赖 `TX_NOOP`；`grep -rnE "TODO|FIXME"` 在 `.go`/`.mts`/`.mjs`/`.proto` 下已无命中（`make test` 双模块绿，含 proto 守卫）。commit `3476475`。
- [x] 上面那次改动新增 4 行，导致文档里精确行号引用漂移：`docs/architecture.md:21` 与 `docs/glossary.md:11` 均引用 `internal/sdk/fabric.go:13` 指 `NoopClient`，改动后类型已落在第 17 行。两处已改为 `:17`（`fabric.go:7` 的 `Client` 接口引用仍准确，未动）。
  - 证据：`grep -rn "fabric.go:[0-9]" docs/*.md` 全部与源码实际行号一致（`Client` 在 `:7`、`NoopClient` 在 `:17`）；`docs` 测试 11/11 绿、`pnpm --filter @playsmith/docs build` 绿（114.81s，页面无回归）。commit `c02f744`。
- [x] 既然上面那次编辑就制造了一处行号漂移，说明文档里 `file:line` 引用需要系统核验：对 14 个文档文件提取全部 `路径:行号` 引用与源码实际比对（引用行必须存在；文档就近写明标识符时，该标识符须落在引用行 ±3 行窗口内）。
  - 证据：222 处引用全部通过，**0 处漂移**（唯一两例即上面已修的 `fabric.go:13→17`）；另有 7 处 `../<path>:<line>` 为 `docs/` 源码根下的跨树证据链接（正是 `config.mts` 的 `ignoreDeadLinks: [/^(\.\/)?\.\.\//]` 豁免的那类），按 `docs/` 相对路径逐一确认文件均存在。校验脚本为一次性临时脚本（`/tmp`，未入库）。
- [x] 环境变量口径双向核验：`docs/configuration.md` 声称覆盖全部 env 读取行号，故对「代码 `os.Getenv` 实际读取」与「文档列出」做双向比对。
  - 证据：代码侧（排除生成物）共 12 个 `os.Getenv` 变量——`FABRIC_CERT_PATH`、`FABRIC_CHANNEL_NAME`、`FABRIC_GATEWAY_ENDPOINT`、`FABRIC_KEY_PATH`、`FABRIC_MSP_ID`、`FABRIC_TLS_PATH`、`GRPC_PORT`、`JWT_SECRET`、`PORT`、`POSTGRES_DSN`、`TLS_CERT_PATH`、`TLS_KEY_PATH`，**全部在文档中列出**；反向，文档 env 表中 11 个带下划线的变量（`PORT` 因长度被过滤条件排除，已单独确认有文档）**全部在代码中被读取**。无未文档化的隐式配置，无文档化的幽灵变量。
- [x] [Minor] JWT 整体修复被提前勾选，证据不足以支持完成状态
  - 证据：已按修复方向拆为两项并各自写明范围——按操作授权臂 `[x]`（本文件第 36 行，`5f0c8be`；2026-11-15 起链码侧门禁补于 `e922b90`），身份隔离臂单独标为 `- [Critical-BLOCKED]`（本文件第 75 行，位置 `internal/auth/authorize.go:26` + `cmd/api/main.go:104`，未修）；本条 Minor 只针对勾选范围失真，现已被第 36/75 行的拆分取代。 | 位置 [todo.md:36](/home/cui/workspaces/Playsmith/todo.md:36) | 问题：`5f0c8be` 将身份绑定问题改为 `[x]`，但第 38 行同时写明“未修”；代码证据仅支持 [authorize.go:31](/home/cui/workspaces/Playsmith/internal/auth/authorize.go:31) 的 Mint/BatchMint 角色门禁。当前未提交的第 69 行又重新登记了该风险 | 影响：账本会把尚未完成的身份隔离呈现为已关闭 | 修复方向：拆分角色门禁与身份绑定两项，分别记录准确 SHA、当前位置和验证范围。

## CI 门禁（2026-11-21，补 Go 侧 CI）

- [x] 新增 `.github/workflows/ci.yml`：仓库此前只有 pages（文档站）workflow，`make build`/`make vet`/`make test`（proto 守卫 + 主模块 + chaincode 模块）从未在 CI 强制过。现 push/PR 触发：checkout → setup-go 1.26（cache 关，避免无 go.sum 变更时的缓存噪声）→ apt 装 protoc → `make proto-doctor` 校验工具链 → `make proto-install` → build/vet/test。权限仅 `contents: read`。
  - 证据：run [44859493416](https://github.com/cuihairu/Playsmith/actions/runs/44859493416) `completed/success`（build、vet、test 全绿，test 含 proto 守卫与 chaincode 模块）。commit `df39a79`（初版，失败：`protoc-3.21.12-linux-x86_64.zip` 下载 404——v3.21.12 只有 git tag 没有 GitHub Release 对象，`releases/tags/v3.21.12` 404 而 v22.4 同形 URL 302，可证版本特有缺口）→ `fe19022` 改从 apt 装 `protobuf-compiler`（Ubuntu 24.04 包即 3.21.12-15ubuntu1.x，与本机一致；`make proto-doctor` 钉死版本，apt 漂移会当场红）。
  - 运维备忘：`setup-go@v6.1.0` 会把 `GOPATH/bin` 加进 PATH，故 `make proto-install` 装的 protoc-gen-go / protoc-gen-go-grpc 无需手动追加 PATH；且 setup-go 在 run 环境设 `GOTOOLCHAIN: local`，go.mod 的 `go 1.26` 与安装版本一致即可。

## 依赖公告（2026-11-21，dependabot 推送时上报）

- [Blocked] 文档站 dev 工具链 9 条公告（vite ×8、esbuild ×1；1 high / 7 medium / 1 low）——全部只影响 `pnpm dev` 开发服务器，`vitepress build` 的静态 HTML 产物不含受影响代码。
  - 现状：`docs/package.json` 锁 `vitepress@^1.6.4`（1.6.4 即 npm 的 `latest`），传递依赖锁定 `vite@5.4.21`、`esbuild@0.21.5`；vite 5.x 线最新即为 5.4.21，已无 5.x 线内的修复版，6.x/7.x/8.x 各线才有补丁。
  - BLOCKED 缺什么：需升 vitepress 2.x 才能带上 vite ≥6，属文档站主版本升级。**2026-11-21 复核 npm dist-tags 后结论更硬**：`latest = 1.6.4`（即当前锁定版本）、`next = 2.0.0-alpha.21`——**2.x 至今没有任何稳定版**，所以修复路径不是「升级到正式版」而是「降级到预发布版」，风险更高，明确不做，等 2.x 转正再评估。公告清单：https://github.com/cuihairu/Playsmith/security/dependabot （2026-11-21 查询到 9 条 open）。
    - 为何 dependabot 自己开不了 PR（2026-11-21 查其任务定义）：更新策略为 `security-updates-only: true` 且 `update-subdependencies: false`，而 `vite`/`esbuild`/`launch-editor` 全是 `vitepress` 的传递依赖，直接依赖 `vitepress@^1.6.4` 已无更高 1.x 版本可升——无子依赖更新通道、无直接依赖可升，故 2026-11-14 连续 3 次构建失败、当前 0 个 open PR。受影响范围与锁定版本比对：`esbuild >= 0.17.0 < 0.28.1`（锁定 0.21.5，确受影响，修复需 ≥0.28.1）；多数 `vite` 公告的 5.x 区间上限为 5.4.18/5.4.20，锁定 5.4.21 已在其外，真正待修的是 esbuild 与 launch-editor。