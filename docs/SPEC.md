# Fabric-1155 Specification (Playsmith)

Goal
- Define a Fabric-native protocol that mirrors ERC-1155 semantics for game assets: mint, transfer, approvals, metadata, supply, listings, and trades.
- Provide stable chaincode functions, state layout, events, and API mapping.

Identity
- Owner/operator identifiers are strings. Recommended format: `playerId` or `mspId:subject`.
- Authorization uses Fabric identities: MSP and X.509 attributes (e.g., `role=MINTER`).

State Keys (Composite)
- `bal~{id}~{owner}` -> uint64
- `meta~{id}` -> JSON `{ uri, creator, supplyCap? }`
- `supply~{id}` -> uint64
- `op~{owner}~{operator}` -> "1" or "0"
- `ask~{id}~{seller}~{listingId}` -> JSON `{ amount, price?, priceHash?, ts }`
- Private Data (optional): collection `playsmithPvt`
  - `askpvt~{id}~{seller}~{listingId}` -> JSON `{ price, ts }`

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
- `AskCreated`: `{ id, seller, listingId, amount, price, priceHash }` (price omitted or 0 if private)
- `AskCanceled`: `{ id, seller, listingId }`
- `Trade`: `{ id, seller, buyer, listingId, amount, price, priceHash }` (price omitted or 0 if private)

Authorization
- MINTER: cert attribute `role=MINTER` or MSP allowlist.
- Transfers: invoker must be owner or approved operator.
- Listings: seller must have at least `amount` balance.

Concurrency & Idempotency
- Use listingId = `TxID` by default. Optional external idempotency key can be implemented via `dedup~{extId}`.
- Keys are sharded by `(id, owner)` to minimize MVCC conflicts.

API Mapping (Gateway)
- REST
  - POST `/api/v1/assets/mint` `{ id, to, uri, amount }`
  - POST `/api/v1/assets/batchMint` `{ ids, tos, amounts, uris }`
  - POST `/api/v1/assets/transfer` `{ id, from, to, amount }`
  - POST `/api/v1/assets/batchTransfer` `{ ids, from, to, amounts }`
  - POST `/api/v1/assets/approveAll` `{ owner, operator, approved }`
  - GET  `/api/v1/assets/{id}/balance/{owner}`
  - GET  `/api/v1/assets/{id}/uri`
  - GET  `/api/v1/assets/{id}/exists`
  - GET  `/api/v1/assets/{id}/totalSupply`
  - POST `/api/v1/market/list` `{ id, seller, amount, price }`
  - POST `/api/v1/market/list-private` `{ id, seller, amount, price }` (price sent via transient)
  - POST `/api/v1/market/cancel` `{ id, seller, listingId }`
  - POST `/api/v1/market/buy` `{ id, seller, listingId, buyer, amount }`
- gRPC mirrors the above (omitted here).

Private Data Option
- Enable collection `playsmithPvt` across trading orgs. Client sends `price` in transient map:
  - Key: `price` -> bytes (string or uint64 encoded)
  - Chaincode computes `priceHash=sha256(price)` and publishes it in public state/events.

Endorsement Policy
- Default: `AND('Org1.peer','Org2.peer')`. Use State-Based Endorsement (SBE) for asset-level hardening if needed.

Errors
- 400-range: invalid args, insufficient balance, not approved.
- 404: listing not found.
- 500: state errors.

Compatibility Notes
- Keep event names/fields aligned with ERC-1155 to ease indexer/frontend reuse.
- No gas; apply rate limiting/quotas at API layer.

Versioning
- Contract name: `playsmith-assets`, version v0.1.0.
