# Glossary

This glossary uses responsibility layers rather than directory names. References are one-based `file:line` anchors in the current source tree.

| Layer / Term | English | Definition | Evidence |
|---|---|---|---|
| 接入生命周期 | Access Lifecycle | One process that selects the Fabric client, JWT/TLS options, and REST/gRPC listeners, then serves until process exit. | `cmd/api/main.go:230`, `cmd/api/main.go:252`, `cmd/api/main.go:303` |
| REST 网关 | REST Gateway | The HTTP face that maps JSON requests to Fabric submissions/evaluations and exposes transaction IDs. | `cmd/api/main.go:303`, `cmd/api/main.go:90`, `cmd/api/main.go:216` |
| gRPC 网关 | gRPC Gateway | A generated protobuf contract plus service methods that mirror the REST surface. | `proto/playsmith/v1/gateway.proto:8`, `internal/gateway/service.go:17`, `internal/gatewaypb/gen/gateway.pb.go:1` |
| 传输鉴权 | Transport Authentication | The JWT issuing middleware, bearer checks, and per-operation role authorization (`Mint`/`BatchMint` require `role=MINTER`) shared by REST and gRPC; issuance uses the same configured secret, and an optional `JWT_IDENTITY_MAP` binding additionally gates which validated subjects may act. | `internal/auth/jwt.go:20`, `internal/auth/authorize.go:26`, `internal/auth/identity.go:68`, `cmd/api/main.go:324`, `cmd/api/main.go:341`, `cmd/api/main.go:434` |
| Fabric 适配 | Fabric Adapter | The `sdk.Client` boundary that keeps transport handlers independent of real Fabric Gateway or development no-op behavior. | `internal/sdk/fabric.go:7`, `internal/sdk/gateway.go:26`, `internal/sdk/fabric.go:17` |
| 交易执行 | Transaction Execution | Fabric Gateway submission over mTLS/gRPC, commit-status checking, transaction-ID extraction, and read-only evaluation. | `internal/sdk/gateway.go:33`, `internal/sdk/gateway.go:78`, `internal/sdk/gateway.go:110` |
| 提交错误 | Commit Error | A structured failed-commit result carrying the Fabric transaction ID and validation status code. | `internal/sdk/errors.go:9`, `internal/sdk/errors.go:16` |
| 链码生命周期 | Chaincode Lifecycle | The Hyperledger Fabric contract process instantiated from `GameContract`; ledger code lives in the separate chaincode module. | `chaincode/playsmith-assets/main.go:18`, `chaincode/playsmith-assets/main.go:543`, `chaincode/playsmith-assets/go.mod:1` |
| 资产协议 | Asset Protocol | ERC-1155-style mint, transfer, batch, metadata, supply, and approval operations over Fabric composite keys. | `chaincode/playsmith-assets/main.go:30`, `chaincode/playsmith-assets/main.go:202`, `chaincode/playsmith-assets/main.go:266` |
| 市场协议 | Marketplace Protocol | Public/private listing creation, cancellation, and purchase that atomically mutate balances and listing state. | `chaincode/playsmith-assets/main.go:84`, `chaincode/playsmith-assets/main.go:105`, `chaincode/playsmith-assets/main.go:317` |
| 账本状态 | Ledger State | Composite-key state (`bal`, `meta`, `supply`, `op`, `ask`) written through the Fabric stub. | `chaincode/playsmith-assets/main.go:362`, `chaincode/playsmith-assets/main.go:453`, `docs/SPEC.md:11` |
| 私有数据 | Private Data | The `playsmithPvt` collection holding private listing prices while public state retains a SHA-256 price hash. | `chaincode/playsmith-assets/main.go:317`, `chaincode/playsmith-assets/main.go:321`, `deploy/collections.json:1` |
| 协议事件 | Protocol Events | JSON chaincode events emitted after ledger mutations and consumed as the projector's change feed. | `chaincode/playsmith-assets/main.go:520`, `chaincode/playsmith-assets/main.go:51`, `cmd/indexer/main.go:146` |
| 身份授权 | Ledger Authorization | Ledger-side identity checks using the X.509 `MINTER` attribute, owner/operator approval state, and balance precondition. | `chaincode/playsmith-assets/main.go:374`, `chaincode/playsmith-assets/main.go:381`, `chaincode/playsmith-assets/main.go:392` |
| 投影生命周期 | Projection Lifecycle | The indexer process that connects Postgres and Fabric, then consumes chaincode events until context cancellation or channel close. | `cmd/indexer/main.go:376`, `cmd/indexer/main.go:393`, `cmd/indexer/main.go:415` |
| 派生模型 | Derived Model | The Postgres projection schema for raw events, balances, listings, and trades. | `cmd/indexer/main.go:30`, `cmd/indexer/main.go:108`, `cmd/indexer/main.go:123` |
| 事件投影 | Event Projection | Parsing named JSON events, recording raw events for deduplication, and applying transactional SQL updates. | `cmd/indexer/main.go:146`, `cmd/indexer/main.go:165`, `cmd/indexer/main.go:311` |
| 索引存储 | Index Store | PostgreSQL, configured solely by `POSTGRES_DSN`; the implementation has no Redis, MySQL, or local application file store. | `cmd/indexer/main.go:380`, `cmd/indexer/main.go:384`, `cmd/indexer/main.go:389` |
| 环境配置 | Environment Configuration | Runtime inputs supplied as process environment variables and PEM files read at startup. | `cmd/api/main.go:243`, `internal/sdk/gateway.go:15`, `cmd/indexer/main.go:452` |
| 部署协议 | Deployment Protocol | Fabric collection membership/endorsement policy represented by `deploy/collections.json`. | `deploy/collections.json:1`, `chaincode/playsmith-assets/main.go:338`, `deploy/README.md:1` |
| 前端占位 | Frontend Placeholder | The declared-but-unimplemented UI boundary intended to query the projected PostgreSQL views. | `web/README.md:1`, `web/README.md:3`, `cmd/indexer/main.go:46` |
