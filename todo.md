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
  - 绑定每游戏服 Fabric 身份臂仍 BLOCKED：需真实机器/凭据（Fabric CA 为每游戏服颁发 X.509 证书 + 私钥托管），无法在仓内验证，未修。
- [x] 审核发现：HTTP 和 gRPC 均监听明文连接，直接部署时签发密钥及 bearer token 会在网络中明文传输，泄露后可伪造令牌或冒用调用者（cmd/api/main.go:336）。
  - 证据：`cmd/api/main.go` `serveGRPC` 在设 `TLS_CERT_PATH`/`TLS_KEY_PATH` 时用 `credentials.NewServerTLSFromFile` 加 `grpc.Creds`；HTTP 用 `http.ListenAndServeTLS`；未配置时回退明文（开发模式）；`tlsPaths()` 读环境变量；测试 `TestTLSPaths` 通过。
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

## Codex 深审发现（2026-11-13，luna profile 近5 commit 审核，待修）

- [Critical] Buy 不限制订单成交数量 | 位置 [chaincode/playsmith-assets/main.go:107](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:107) | 问题：解析了 `ask.Amount`，却直接用请求的 `amount` 扣卖家余额，之后无条件 `DelState(askKey)`。历史残留 | 影响：卖单数量为 1、卖家余额为 100 时，请求购买 100 仍可成功；少量购买也会删除整个剩余订单。公开、私有及旧格式订单均受影响 | 修复方向：明确整单或部分成交语义，校验数量并正确保留余量；补超额及部分成交测试。
- [Critical] JWT 身份隔离仍未闭环 | 位置 [internal/auth/authorize.go:26](/home/cui/workspaces/Playsmith/internal/auth/authorize.go:26)、[cmd/api/main.go:104](/home/cui/workspaces/Playsmith/cmd/api/main.go:104) | 问题：`Authorize` 只要求非空 MSP/role，并限制铸造角色；通过后直接调用共享的 `s.Fabric.Submit(...)`，未传递或使用 subject/MSP。`5f0c8be` 修复了 MINTER 门禁，但未解决身份共享 | 影响：任意有效 PLAYER 令牌仍可转移共享证书拥有或获授权的资产；不同游戏服无法隔离权限。新增测试仅用 fake client，未覆盖此路径 | 修复方向：根据可信身份映射选择 Fabric 签名身份，或实现链上可验证的委托授权；补跨 subject/MSP 的隔离测试。
- [Critical] 任意调用方可替资产所有者授权 | 位置 [chaincode/playsmith-assets/main.go:192](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:192) | 问题：`SetApprovalForAll` 仅检查参数非空，随后直接 `PutState(key, []byte(v))`；没有验证调用者是否为 owner。历史残留 | 影响：攻击者可先将自己或共享网关证书设为任意 owner 的 operator，再调用 Transfer 盗取资产；已有转账鉴权因此可被绕过 | 修复方向：只有经验证的 owner 才能修改授权；补第三方自授权失败及状态不变测试。
- [Critical] 余额和供应量存在 uint64 溢出 | 位置 [chaincode/playsmith-assets/main.go:342](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:342)、[main.go:371](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:371) | 问题：两处均直接执行 `strconv.FormatUint(n+amt, 10)`。历史同型残留 | 影响：余额或供应量达到 `MaxUint64` 后再加 1 会写成 0；合法角色门禁不能保护账本数值完整性 | 修复方向：加法前检查 `amt > MaxUint64-n`，失败时拒绝整个交易；补余额与供应量边界测试。
- [Critical] 未经卖家授权即可上架、撤单并串联盗取资产 | 位置 [chaincode/playsmith-assets/main.go:77](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:77)、[main.go:240](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:240)、[main.go:266](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:266) | 问题：公开/私有上架只执行 `requireBalance(...)`，撤单直接 `DelState(askKey)`，均不校验 seller 身份。历史同型残留 | 影响：任意调用者可冒充持币卖家创建订单，再通过 Buy 将资产转给自己；也可撤销他人订单 | 修复方向：在三个入口验证卖家或其合法代理权限；补不同身份之间的拒绝测试。
- [Critical] 自转账会增发余额，批量交易会覆盖前序写入 | 位置 [chaincode/playsmith-assets/main.go:66](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:66)、[main.go:172](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:172) | 问题：先 `putSubBalance` 再 `putAddBalance`，批量操作又循环调用单笔方法；所用 Fabric shim 的 `GetState` 不读取本交易尚未提交的写集。历史残留 | 影响：余额 100 自转 10，最终写入 110；Buy 的 seller=buyer 同样受影响。批量重复 token/owner 会丢失前序增减，同 token 向不同接收者铸造也会算错供应量。MockStub 立即更新 State，掩盖了真实语义差异 | 修复方向：自转账保持余额不变；批量先聚合变化，再逐键读取、校验和写入一次；使用模拟真实写集语义的测试。
- [Major] BatchMint 事件丢失真实接收者 | 位置 [chaincode/playsmith-assets/main.go:175](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/main.go:175)、[cmd/indexer/main.go:179](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:179) | 问题：最终事件写为 `"to": "batch"`，Indexer 将其作为所有资产的实际 owner；真实 Fabric 每交易只保留一个事件，循环内的 TransferSingle 会被覆盖 | 影响：批量铸造被索引到虚构的 batch 账户。`TestTransferBatch` 恰好使用 `"to":"batch"` 且仅检查执行次数，未校验真实业务结果 | 修复方向：批量铸造事件携带逐项接收者，并同步修改消费协议和断言。
- [Major] Indexer 数值链路丢精度且不能承载 uint64 | 位置 [cmd/indexer/main.go:124](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:124)、[main.go:155](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:155) | 问题：事件解码为 `map[string]any`，数量和价格经 `float64` 再转 `int64`，数据库使用 BIGINT；上架、成交及批量路径存在同型转换 | 影响：`9007199254740993` 会被舍入为 `9007199254740992`，超过 int64 范围的合法链码数值也无法正确保存 | 修复方向：采用精确数字解码和范围校验，数据库使用能完整表达 uint64 的数值类型；补大数测试。
- [Major] Indexer 的扣款 SQL 引用了不存在的列 | 位置 [cmd/indexer/main.go:160](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:160)、[main.go:231](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:231) | 问题：两处更新包含 `updated_at=now()`，但 `balances` 建表只有 `token_id`、`owner`、`balance`。历史残留，`todo.md:29` 已勾选实现完成 | 影响：按仓库 schema 创建的新数据库无法投影普通转账和成交，事务会回滚；mockExec 总返回成功，现有测试不能发现 SQL 错误 | 修复方向：统一 schema 与 SQL，并提供已有数据库迁移及真实数据库回归测试。
- [Major] TLS 配置不完整时降级为明文 | 位置 [cmd/api/main.go:269](/home/cui/workspaces/Playsmith/cmd/api/main.go:269)、[main.go:390](/home/cui/workspaces/Playsmith/cmd/api/main.go:390) | 问题：HTTP/gRPC 都只有在 `certFile != "" && keyFile != ""` 时启用 TLS，任一配置遗漏即走明文；`TestTLSPaths` 只验证环境变量读取 | 影响：原本意图启用 TLS 的部署可能暴露 bearer token 及签发密钥；`todo.md:39` 的完成勾选未覆盖此异常路径 | 修复方向：证书和私钥只配置一个时拒绝启动；补两种缺失配置测试。
- [Major] 事件处理失败和重启会永久漏账 | 位置 [cmd/indexer/main.go:280](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:280)、[main.go:340](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:340) | 问题：处理错误仅 `log.Printf(...)` 后消费下一条；订阅 `ChaincodeEvents(ctx, chaincodeName)` 未指定起点或 checkpoint，所用 SDK 默认从下一次提交开始 | 影响：暂时性数据库错误直接丢事件，初次启动及停机期间的历史交易不会补齐；流关闭后进程也直接退出 | 修复方向：持久化成功消费位置、恢复订阅，并对失败事件重试，禁止跳过后推进位置。
- [Major] 依赖升级未完成 genproto 拆包迁移 | 位置 [chaincode/playsmith-assets/go.mod:28](/home/cui/workspaces/Playsmith/chaincode/playsmith-assets/go.mod:28) | 问题：`34658f3` 将 gRPC 升至 `v1.85.0`，却保留包含 `googleapis/rpc/status` 的旧整体模块 `genproto@…20230110…`；新版 gRPC 又依赖独立的 `genproto/googleapis/rpc@…20260817…`。只读 `go mod graph` 确认两者同时进入依赖图 | 影响：同一导入路径存在双重提供者，依赖补齐后会形成 ambiguous import，不能据版本号升级认定链码构建已验证 | 修复方向：升级整体 genproto 至拆包后的兼容版本，整理模块及校验和，并单独构建、测试链码模块。
- [Major] 原始事件去重未阻止重复余额变更 | 位置 [cmd/indexer/main.go:129](/home/cui/workspaces/Playsmith/cmd/indexer/main.go:129) | 问题：`INSERT ... ON CONFLICT DO NOTHING` 后未检查插入是否成功，仍继续执行 `handleTransfer` 或 `handleTrade` | 影响：重放同一事件时，原始记录只有一份，余额却重复扣增；成交记录的去重也无法撤销已经执行的余额修改 | 修复方向：仅在首次插入事件时执行投影，并在同一事务内完成；补重复投递测试。
- [Major] 标准测试入口遗漏整个链码模块 | 位置 [Makefile:32](/home/cui/workspaces/Playsmith/Makefile:32) | 问题：`test` 只执行根模块的 `go test ./...`，不会进入具有独立 go.mod 的 `chaincode/playsmith-assets`；build、vet 同样遗漏 | 影响：`04f4fad`/`2e92e6c` 引入的标准检查入口无法验证核心链码，也无法发现 `34658f3` 对链码依赖造成的回归 | 修复方向：三个入口显式检查两个 Go 模块。
- [Minor] JWT 整体修复被提前勾选，证据不足以支持完成状态 | 位置 [todo.md:36](/home/cui/workspaces/Playsmith/todo.md:36) | 问题：`5f0c8be` 将身份绑定问题改为 `[x]`，但第 38 行同时写明“未修”；代码证据仅支持 [authorize.go:31](/home/cui/workspaces/Playsmith/internal/auth/authorize.go:31) 的 Mint/BatchMint 角色门禁。当前未提交的第 69 行又重新登记了该风险 | 影响：账本会把尚未完成的身份隔离呈现为已关闭 | 修复方向：拆分角色门禁与身份绑定两项，分别记录准确 SHA、当前位置和验证范围。