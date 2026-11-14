# Features and Routes

This page lists the routes that actually exist. The REST table is taken from the
gateway router at `cmd/api/main.go:282-296`, where every route is wrapped in the
auth middleware. Request field names are the JSON tags of the structs at
`cmd/api/main.go:33-86`.

## REST routes

| Method | Path | Body | Chaincode operation |
|---|---|---|---|
| POST | `/api/v1/auth/token` | `{ subject, mspId, role }` | none — issues a JWT, guarded by `X-Issue-Secret` |
| POST | `/api/v1/assets/mint` | `{ tokenId, to, uri, amount }` | `Mint` — JWT must carry `role=MINTER` |
| POST | `/api/v1/assets/batchMint` | `{ ids, tos, amounts, uris }` | `BatchMint` — JWT must carry `role=MINTER` |
| POST | `/api/v1/assets/transfer` | `{ tokenId, from, to, amount }` | `Transfer` |
| POST | `/api/v1/assets/batchTransfer` | `{ ids, from, to, amounts }` | `BatchTransfer` |
| POST | `/api/v1/assets/approveAll` | `{ owner, operator, approved }` | `SetApprovalForAll` |
| POST | `/api/v1/market/list` | `{ tokenId, seller, amount, price }` | `ListForSale` |
| POST | `/api/v1/market/list-private` | `{ tokenId, seller, amount, price }` | `ListForSalePrivate` — price goes to transient |
| POST | `/api/v1/market/cancel` | `{ tokenId, seller, listingId }` | `CancelListing` |
| POST | `/api/v1/market/buy` | `{ tokenId, seller, listingId, buyer, amount }` | `Buy` |
| GET | `/api/v1/assets/{tokenId}/uri` | — | `URI` |
| GET | `/api/v1/assets/{tokenId}/exists` | — | `Exists` |
| GET | `/api/v1/assets/{tokenId}/totalSupply` | — | `TotalSupply` |
| GET | `/api/v1/assets/{tokenId}/balance/{owner}` | — | `BalanceOf` |

The four asset reads share one prefix handler: `assetRoute` at
`cmd/api/main.go:428` splits the path after `/api/v1/assets/` and dispatches on
the remaining segments. The listing browser (`listings`), the web UI and
Caliper-style load tests are planned over the indexer projection and are not
routes yet.

Submissions write `{ "result": ..., "txId": "..." }`; failures write HTTP 500
with `{ "error": "..." }` (`cmd/api/main.go:214-222`). Upstream errors are not
yet mapped to 400/404 — that mapping is a spec target in `docs/SPEC.md`, not
current behaviour.

## gRPC face

Setting `GRPC_PORT` starts a second listener on the same `Server`. The contract is
`proto/playsmith/v1/gateway.proto`, with generated bindings in
`internal/gatewaypb/gen`. It mirrors the REST surface and returns the transaction
result plus `tx_id`, so a call can be traced even when the chaincode returns no
value. Block numbers and event receipts are planned; the current response carries
only `tx_id` and `result`. `GRPC_PORT` requires `JWT_SECRET` — the process
refuses to start without it.

## Ledger protocol

State is written under composite keys, which sharding keeps hot spots narrow:

| Key | Value |
|---|---|
| `bal~{id}~{owner}` | uint64 balance, decimal string |
| `meta~{id}` | `{ tokenId, uri, creator, supplyCap? }` |
| `supply~{id}` | uint64 total supply |
| `op~{owner}~{operator}` | `"1"` / `"0"` approval |
| `ask~{id}~{seller}~{listingId}` | `{ amount, price?, priceHash?, ts }` |
| `askpvt~{id}~{seller}~{listingId}` | `{ price, ts }` inside collection `playsmithPvt` |

Minting requires the X.509 attribute `role=MINTER`; transfers require owner or
approved-operator authority; listings require the seller to hold at least the
listed amount. Private listings hash the price into public state and keep the raw
value in the collection, so settlement evidence never leaves the trading orgs.

Chaincode events (`TransferSingle`, `TransferBatch`, `ApprovalForAll`, `URI`,
`AskCreated`, `AskCanceled`, `Trade`) are the indexer's change feed — see
[the architecture notes](/architecture#5-event-projection) for how they are
projected.
