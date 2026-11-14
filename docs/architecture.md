# Architecture

These layers describe runtime responsibilities and state ownership. Directory layout is evidence, not the definition of a layer. All runtime state is remote: chaincode uses Fabric world state and private data; the indexer uses PostgreSQL. The implementation has no Redis, MySQL, in-process durable cache, or application file store.

## 1. Client Boundary

`cmd/api` owns process startup and the public access surface. It selects a real Fabric Gateway client when `FABRIC_GATEWAY_ENDPOINT` is set and otherwise selects `NoopClient`; JWT is enabled only when `JWT_SECRET` is present, and an optional `GRPC_PORT` starts a second listener that requires JWT. REST routes assemble all request-to-chaincode mappings (`cmd/api/main.go:226`, `cmd/api/main.go:282`). The HTTP writer normalizes successful responses as `{result, txId}` and current errors as HTTP 500 `{error}` (`cmd/api/main.go:214`).

REST request schemas are Go structs in `cmd/api/main.go:33`–`cmd/api/main.go:86`. They have no standalone configuration file. The dedicated external protocol is [`proto/playsmith/v1/gateway.proto`](../proto/playsmith/v1/gateway.proto:8), and generated protobuf/grpc bindings live in [`internal/gatewaypb/gen/gateway.pb.go`](../internal/gatewaypb/gen/gateway.pb.go:1) and [`internal/gatewaypb/gen/gateway_grpc.pb.go`](../internal/gatewaypb/gen/gateway_grpc.pb.go:1). Product-facing REST/gRPC wording is in [`README.md`](../README.md:70) and [`docs/SPEC.md`](SPEC.md:55); deployment wording is in [`deploy/README.md`](../deploy/README.md:9). [`web/README.md`](../web/README.md:1) is only a frontend placeholder.

## 2. Access Authentication

`internal/auth` owns the platform-hosted JWT claim shape, HS256 issuance, and validation. Claims carry subject, `mspId`, and `role`; the manager enforces a secret of at least 32 bytes, positive TTL, issuer/audience checks, required expiration, and HMAC-only signing (`internal/auth/jwt.go:14`, `internal/auth/jwt.go:31`, `internal/auth/jwt.go:42`, `internal/auth/jwt.go:63`). Its schema is therefore source-defined, not a file schema.

REST token issuance compares `X-Issue-Secret` with `JWT_SECRET`, issues a one-hour token, and rejects the endpoint when JWT is disabled (`cmd/api/main.go:298`, `cmd/api/main.go:250`). REST middleware applies bearer validation and the shared per-operation authorization after excluding the token endpoint or when JWT is disabled (`cmd/api/main.go:320`, `internal/auth/authorize.go:26`). gRPC requires one authorization metadata value, validates it, and enforces the same per-operation rules (empty `role`/`mspId` rejected; `Mint`/`BatchMint` require `role=MINTER`) (`cmd/api/main.go:403`, `internal/auth/authorize.go:26`). TLS paths are `TLS_CERT_PATH`/`TLS_KEY_PATH`; when both are present REST and gRPC use TLS, otherwise both fall back to plaintext (`cmd/api/main.go:269`, `cmd/api/main.go:464`).

The implementation does not bind JWT claims to distinct Fabric signing identities: one process-wide `GatewayClient` identity submits all calls (`cmd/api/main.go:228`). `todo.md:69` already records this open authorization gap.

## 3. Fabric Adapter

`internal/sdk` isolates the application process from Hyperledger Fabric. `Client` defines submit, transient submit, and evaluate contracts (`internal/sdk/fabric.go:7`). `NoopClient` returns development fixtures without touching Fabric (`internal/sdk/fabric.go:13`). `GatewayClient` loads X.509 identity and TLS PEM files, constructs an X.509 identity and signer, opens a TLS gRPC connection to the configured gateway, and selects the configured channel (`internal/sdk/gateway.go:33`). Its schema is `sdk.Config` (`internal/sdk/gateway.go:16`), populated exclusively by API environment variables (`cmd/api/main.go:228`).

Submissions wait for commit status and return both payload and Fabric transaction ID (`internal/sdk/gateway.go:78`, `internal/sdk/gateway.go:94`). Failed commits become `CommitError`, retaining transaction ID and validation code (`internal/sdk/errors.go:9`). There is no dedicated protocol file here; the adapter speaks Hyperledger Fabric Gateway rather than the repository's public gateway proto.

## 4. Ledger Contract

`chaincode/playsmith-assets` is an independent Go module containing the on-ledger protocol and execution state. `GameContract` is the Fabric contract entry; `main` instantiates and starts the chaincode (`chaincode/playsmith-assets/main.go:14`, `chaincode/playsmith-assets/main.go:397`). Its public protocol specification is `docs/SPEC.md:11`, while `chaincode/playsmith-assets/go.mod:1` is the module schema/dependency source.

Asset operations enforce and mutate ledger state: mint requires the `MINTER` attribute and writes balance, metadata, supply, and a mint event (`chaincode/playsmith-assets/main.go:27`); transfer requires owner/operator authority (`chaincode/playsmith-assets/main.go:58`); batches call these operations and emit batch events (`chaincode/playsmith-assets/main.go:165`, `chaincode/playsmith-assets/main.go:179`); approvals and queries use `op`, `meta`, and `supply` keys (`chaincode/playsmith-assets/main.go:191`, `chaincode/playsmith-assets/main.go:210`, `chaincode/playsmith-assets/main.go:221`). Composite-key construction and balance helpers centralize world-state access (`chaincode/playsmith-assets/main.go:277`, `chaincode/playsmith-assets/main.go:328`).

Market operations split listing privacy. Public listings store amount/price/timestamp (`chaincode/playsmith-assets/main.go:76`); buy verifies the listing, mutates both balances, deletes it, and emits a trade event (`chaincode/playsmith-assets/main.go:94`). Private listings read transient `price`, hash it, publish the hash publicly, and store the raw price in `playsmithPvt` (`chaincode/playsmith-assets/main.go:239`). Cancel is intentionally best-effort for private state (`chaincode/playsmith-assets/main.go:265`). Ledger authorization rules are in `chaincode/playsmith-assets/main.go:289`.

Configuration is deployment identity, not environment variables: the contract reads client identity attributes and transaction context, while collection policy comes from `deploy/collections.json:1`. Protocol/copy sources are `docs/SPEC.md:20`, `docs/SPEC.md:37`, and the Chinese implementation summary in `README.md:53`.

## 5. Event Projection

`cmd/indexer` is the sole remote derived-state writer. Startup requires `POSTGRES_DSN`, connects to PostgreSQL, executes idempotent DDL, then connects Fabric and consumes its event channel until context cancellation or channel close (`cmd/indexer/main.go:247`). The process schema is `cmd/indexer/main.go:290`; the persisted projection schema is embedded DDL at `cmd/indexer/main.go:21`.

`Indexer` owns transactional event application. Each real event begins/commits/rolls back one PostgreSQL transaction, while tests can inject an exec-only recorder (`cmd/indexer/main.go:85`, `cmd/indexer/main.go:100`). The router parses JSON, upserts the raw event keyed by block/transaction/event for deduplication, then dispatches five protocol events (`cmd/indexer/main.go:123`). Handlers project transfer, listing, cancellation, and trade changes (`cmd/indexer/main.go:151`, `cmd/indexer/main.go:194`, `cmd/indexer/main.go:211`, `cmd/indexer/main.go:220`). There is no exclusive protocol file; the shared contract is the JSON event payload documented at `docs/SPEC.md:37` and emitted at `chaincode/playsmith-assets/main.go:374`. User-facing copy is in `README.md:94` and `README.md:117`.

## 6. Deployment Contract

`deploy` is a deployment contract layer rather than executable infrastructure code. `deploy/collections.json:1` defines the private-data collection name, two-organization read/write membership, propagation quorum, lifetime, and endorsement policy consumed when deploying the chaincode. `deploy/README.md:1` describes local development commands and optional JWT/gRPC setup; `README.md:97` is the broader local Fabric quick start. There is no schema-definition system: JSON is the contract, Markdown is human guidance.

## Layering Rationale

1. Public access and process assembly are split because the same process can choose a no-op or real Fabric backend and optional transport/security listeners without changing handlers (`cmd/api/main.go:228`, `cmd/api/main.go:242`, `cmd/api/main.go:258`).
2. HTTP and gRPC share the Fabric client but need separate protocol encodings and middleware: REST uses JSON structs and `http.ServeMux`, while gRPC uses generated service bindings (`cmd/api/main.go:282`, `proto/playsmith/v1/gateway.proto:8`, `internal/gatewaypb/gen/gateway.pb.go:1`).
3. Authentication is separate from handlers because JWT issuance, HMAC validation, and time/issuer/audience policy form one reusable component (`internal/auth/jwt.go:31`, `internal/auth/jwt.go:42`, `internal/auth/jwt.go:63`).
4. Transport authentication and ledger authorization must not be conflated: JWT check precedes the Fabric call, while mint role, transfer owner/operator, and seller balance are enforced from Fabric identity and ledger state (`cmd/api/main.go:320`, `chaincode/playsmith-assets/main.go:296`, `chaincode/playsmith-assets/main.go:307`, `chaincode/playsmith-assets/main.go:289`).
5. The `sdk.Client` boundary exists to support the documented real/no-op startup switch and to make both REST and gRPC testable against the same fake (`cmd/api/main.go:228`, `internal/sdk/fabric.go:7`, `cmd/api/main_test.go:1`).
6. Transaction execution is separated from adapter configuration because connection/identity setup happens once, while submit/evaluate occurs per request (`internal/sdk/gateway.go:33`, `internal/sdk/gateway.go:78`).
7. Ledger contract code is a separate module and process because Fabric deploys the directory as chaincode, using a distinct dependency graph (`chaincode/playsmith-assets/go.mod:1`, `chaincode/playsmith-assets/main.go:397`, `README.md:106`).
8. Ledger contract code is distinct from the API project so it can define its own ledger key protocol (`bal`, `meta`, `supply`, `op`, `ask`) and identity checks without importing application types (`chaincode/playsmith-assets/main.go:277`, `chaincode/playsmith-assets/main.go:296`, `docs/SPEC.md:11`).
9. Asset and marketplace operations stay in one Fabric contract because they are not independently deployable: listing creation requires balance state and purchase atomically writes balances, listing state, and events (`chaincode/playsmith-assets/main.go:81`, `chaincode/playsmith-assets/main.go:126`, `chaincode/playsmith-assets/main.go:132`).
10. Private listing logic is an explicit sublayer because transient input, public hash, and private collection storage have different visibility and deployment requirements (`chaincode/playsmith-assets/main.go:243`, `chaincode/playsmith-assets/main.go:251`, `deploy/collections.json:1`).
11. Event projection is separated from synchronous write responses because Fabric transactions return commit result/TxID immediately, while browseable balances/listings/trades are asynchronously derived from JSON events (`internal/sdk/gateway.go:78`, `cmd/indexer/main.go:123`).
12. Raw-event storage is retained inside the projection so deduplication evidence survives independently of later model rewrites (`cmd/indexer/main.go:129`).
13. PostgreSQL uses one transaction per event because several event types touch multiple model rows and must succeed or fail together (`cmd/indexer/main.go:108`, `cmd/indexer/main.go:220`, `cmd/indexer/main.go:231`).
14. The deployment contract is separate because collection membership/endorsement policy is network deployment input, not chaincode runtime code (`deploy/collections.json:1`, `chaincode/playsmith-assets/main.go:260`).
15. The frontend is kept as a separate placeholder because the repository's current query contract is only projected PostgreSQL schema plus REST plans; no browser implementation exists (`cmd/indexer/main.go:32`, `web/README.md:1`, `README.md:148`).
