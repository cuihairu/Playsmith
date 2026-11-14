# Configuration

Every setting is a process environment variable plus a few PEM file paths. There
is no configuration file to edit, and nothing is persisted by the application.

## API gateway (`cmd/api`)

| Variable | Effect | Read at |
|---|---|---|
| `FABRIC_GATEWAY_ENDPOINT` | When set, the gateway builds a real Fabric Gateway client; when unset it falls back to `NoopClient` dev mode | `cmd/api/main.go:228` |
| `FABRIC_MSP_ID` | MSP of the gateway's own Fabric identity | `cmd/api/main.go:230` |
| `FABRIC_CERT_PATH` | X.509 certificate for that identity | `cmd/api/main.go:231` |
| `FABRIC_KEY_PATH` | Private key for that identity | `cmd/api/main.go:232` |
| `FABRIC_TLS_PATH` | Gateway TLS CA material | `cmd/api/main.go:233` |
| `FABRIC_CHANNEL_NAME` | Channel the transactions are submitted to | `cmd/api/main.go:235` |
| `JWT_SECRET` | When set (≥32 bytes) JWT auth is on; also the secret `POST /api/v1/auth/token` checks against | `cmd/api/main.go:250` |
| `GRPC_PORT` | Starts the gRPC listener; refuses to start unless `JWT_SECRET` is set | `cmd/api/main.go:258` |
| `TLS_CERT_PATH` | REST/gRPC served over TLS when **both** this and `TLS_KEY_PATH` are set | `cmd/api/main.go:269`, `:464` |
| `TLS_KEY_PATH` | Same pair as above; a half-configured pair falls back to plaintext | `cmd/api/main.go:269`, `:464` |
| `PORT` | HTTP listen port, default `8080` | `cmd/api/main.go:246-249` |

The chaincode name is fixed to `playsmith-assets` at `cmd/api/main.go:236`.

Two caveats worth knowing before you deploy:

- **The Fabric identity is shared.** One `GatewayClient` is constructed at startup
  and every caller submits through it (`cmd/api/main.go:228`). JWT claims are
  checked and gated per operation, but they are not yet bound to distinct Fabric
  signing identities. `todo.md:69` tracks that gap.
- **TLS is all-or-nothing.** Only a complete certificate/key pair enables TLS on
  both faces; leaving one side unset serves plaintext, which is only acceptable
  in development.

`X-Issue-Secret` is a request header, not an environment variable: it must equal
`JWT_SECRET` for token issuance to succeed (`cmd/api/main.go:298-306`).

## Indexer (`cmd/indexer`)

| Variable | Effect | Read at |
|---|---|---|
| `POSTGRES_DSN` | Required. Missing DSN aborts startup | `cmd/indexer/main.go:251` |
| `FABRIC_GATEWAY_ENDPOINT` | Fabric endpoint for the event stream | `cmd/indexer/main.go:290` |
| `FABRIC_CERT_PATH` | Identity certificate | `cmd/indexer/main.go:291` |
| `FABRIC_KEY_PATH` | Identity private key | `cmd/indexer/main.go:292` |
| `FABRIC_TLS_PATH` | Gateway TLS material | `cmd/indexer/main.go:293` |
| `FABRIC_CHANNEL_NAME` | Channel to subscribe | `cmd/indexer/main.go:294` |
| `FABRIC_MSP_ID` | MSP of the subscribing identity | `cmd/indexer/main.go:295` |

The projection schema is embedded DDL applied idempotently at startup
(`cmd/indexer/main.go:21`); there is no migration file.

## Deployment contract

`deploy/collections.json` is the only deployment input: it declares the
`playsmithPvt` collection, its two-organization read/write membership, quorum,
lifetime and endorsement policy. `deploy/README.md` covers the local
fabric-samples/test-network walkthrough.
