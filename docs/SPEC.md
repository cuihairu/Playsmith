# Fabric-1155 Specification (Playsmith)

Goal
- Define a Fabric-native protocol that mirrors ERC-1155 semantics for game assets: mint, transfer, approvals, metadata, supply, listings, and trades.
- Provide stable chaincode functions, state layout, events, and API mapping.

Identity
- Owner/operator identifiers are strings. Recommended format: `playerId` or `mspId:subject`.
- Authorization uses Fabric identities: MSP and X.509 attributes (e.g., `role=MINTER`).

State Keys (Composite)
- `bal~{id}~{owner}` -> uint64 (decimal string on the ledger)
- `meta~{id}` -> JSON `{ tokenId, uri, creator, supplyCap? }`
- `supply~{id}` -> uint64 (decimal string)
- `op~{owner}~{operator}` -> "1" or "0"
- `ask~{id}~{seller}~{listingId}` -> public listing: `"<price>:<amount>"` (decimal string); private listing: JSON `{ amount, priceHash }`
- Private Data (optional): collection `playsmithPvt`
  - `askpvt~{id}~{seller}~{listingId}` -> JSON `{ price }`

Transactions (Chaincode)
- Assets
  - `Mint(id, to, uri, amount)` -> void (MINTER only)
  - `BatchMint(idsJSON, tosJSON, amountsJSON, urisJSON)` -> void
  - `Transfer(id, from, to, amount)` -> void
  - `BatchTransfer(idsJSON, from, to, amountsJSON)` -> void
  - `SetApprovalForAll(owner, operator, approved)` -> void
  - `IsApprovedForAll(owner, operator)` -> bool
  - `URI(id)` -> string
  - `TotalSupply(id)` -> uint64
  - `Exists(id)` -> bool
- Marketplace
  - `ListForSale(id, seller, amount, price)` -> listingId (public price)
  - `ListForSalePrivate(id, seller, amount)` -> listingId (price from transient, stored in PDC, public state stores priceHash)
  - `CancelListing(id, seller, listingId)` -> void
  - `Buy(id, seller, listingId, buyer, amount)` -> void (works for both public/private listings)

Events (JSON payloads)
- `TransferSingle`: `{ operator, from, to, id, value }`
- `TransferBatch`: `{ operator, from, to, ids, values }`
- `ApprovalForAll`: `{ owner, operator, approved }`
- `URI`: `{ id, uri }`
- `AskCreated`: `{ id, seller, listingId, amount, price, priceHash }` (public listing: no `priceHash` key; private listing: `price` emitted as `0` and `priceHash` present)
- `AskCanceled`: `{ id, seller, listingId }`
- `Trade`: `{ id, seller, buyer, listingId, amount }` (current chaincode emits no `price`/`priceHash` fields on this event, for public and private listings alike)

Authorization (spec target — **not yet enforced** in the current chaincode; the code carries TODOs at Mint/Transfer, see `chaincode/playsmith-assets/main.go`)
- MINTER: cert attribute `role=MINTER` or MSP allowlist. (TODO in `Mint`.)
- Transfers: invoker must be owner or approved operator. (TODO in `Transfer`.)
- Listings: seller must have at least `amount` balance. (Not checked in `ListForSale`.)

Concurrency & Idempotency
- Use listingId = `TxID` by default. Optional external idempotency key can be implemented via `dedup~{extId}`.
- Keys are sharded by `(id, owner)` to minimize MVCC conflicts.

API Mapping (Gateway)
- REST (request bodies use the gateway's JSON field names, `tokenId` etc.; `GET /api/v1/assets/{id}/balance/{owner}` is currently dead — the route parser never matches it and always answers 404, tracked in `todo.md`)
  - POST `/api/v1/assets/mint` `{ tokenId, to, uri, amount }`
  - POST `/api/v1/assets/batchMint` `{ ids, tos, amounts, uris }`
  - POST `/api/v1/assets/transfer` `{ tokenId, from, to, amount }`
  - POST `/api/v1/assets/batchTransfer` `{ ids, from, to, amounts }`
  - POST `/api/v1/assets/approveAll` `{ owner, operator, approved }`
  - GET  `/api/v1/assets/{id}/balance/{owner}` (see the dead-route note above)
  - GET  `/api/v1/assets/{id}/uri`
  - GET  `/api/v1/assets/{id}/exists`
  - GET  `/api/v1/assets/{id}/totalSupply`
  - POST `/api/v1/market/list` `{ tokenId, seller, amount, price }`
  - POST `/api/v1/market/list-private` `{ tokenId, seller, amount, price }` (price sent via transient)
  - POST `/api/v1/market/cancel` `{ TokenID, Seller, ListingID }` (this handler's body struct has no json tags — Go default capitalized keys)
  - POST `/api/v1/market/buy` `{ tokenId, seller, listingId, buyer, amount }`
- gRPC: **not implemented**; planned to mirror the REST surface (returns TxID, block number and event receipts).

Private Data Option
- Enable collection `playsmithPvt` across trading orgs. Client sends `price` in transient map:
  - Key: `price` -> bytes (string or uint64 encoded)
  - Chaincode computes `priceHash=sha256(price)` and publishes it in public state/events.

Endorsement Policy
- Default: `AND('Org1.peer','Org2.peer')`. Use State-Based Endorsement (SBE) for asset-level hardening if needed.

Errors (actual behavior)
- Chaincode returns plain error strings (`invalid args`, `insufficient balance`, `listing not found`, `length mismatch`, `missing transient price`).
- The current API Gateway maps every upstream error to HTTP 500 with `{"error": ...}`; the 400/404 mapping below is the spec target, not implemented.
- Spec target: 400-range invalid args / insufficient balance / not approved; 404 listing not found; 500 state errors.

Compatibility Notes
- Keep event names/fields aligned with ERC-1155 to ease indexer/frontend reuse.
- No gas; apply rate limiting/quotas at API layer.

Versioning
- Contract name: `playsmith-assets`, version v0.1.0.
