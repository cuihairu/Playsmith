---
layout: home
hero:
  name: Playsmith
  text: Game assets on a permissioned ledger
  tagline: ERC-1155-style minting, transfer and marketplace on Hyperledger Fabric, with an event-driven PostgreSQL projection
  actions:
    - theme: brand
      text: Features and routes
      link: /features
    - theme: alt
      text: Configuration
      link: /configuration
features:
  - title: Ledger protocol in one contract
    details: Mint, transfer, batch, approvals, metadata, supply and listings live in the `playsmith-assets` chaincode module. Composite keys (`bal`, `meta`, `supply`, `op`, `ask`) keep writes small so transactions do not collide on shared state.
  - title: Two caller faces
    details: A JSON REST gateway for game servers and services, plus an optional gRPC face generated from `proto/playsmith/v1/gateway.proto`. Both return the transaction result and the Fabric transaction ID.
  - title: Per-operation authorization
    details: Platform-hosted JWT claims carry `mspId` and `role`; every authenticated call must carry both, and `Mint`/`BatchMint` additionally require `role=MINTER`, the same gate the chaincode enforces.
  - title: Events become a query model
    details: The indexer consumes chaincode JSON events and projects them into PostgreSQL, so web and statistics reads never scan the ledger. Private listing prices stay in the `playsmithPvt` collection.
---

Playsmith is built around three processes and one contract. `cmd/api` serves callers, `cmd/indexer` keeps a derived view up to date, and `chaincode/playsmith-assets` owns the on-ledger protocol. All runtime state is remote: the ledger plus PostgreSQL. There is no Redis, MySQL, in-process durable cache or application file store.

For the source-level explanation, read the [architecture notes](/architecture) and the [glossary](/glossary).
