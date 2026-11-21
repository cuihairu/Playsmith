# Configuration

Every setting is a process environment variable plus a few PEM file paths. There
is no configuration file to edit, and nothing is persisted by the application.

## API gateway (`cmd/api`)

| Variable | Effect | Read at |
|---|---|---|
| `FABRIC_GATEWAY_ENDPOINT` | When set, the gateway builds a real Fabric Gateway client; when unset it falls back to `NoopClient` dev mode | `cmd/api/main.go:230` |
| `FABRIC_MSP_ID` | MSP of the gateway's own Fabric identity | `cmd/api/main.go:232` |
| `FABRIC_CERT_PATH` | X.509 certificate for that identity | `cmd/api/main.go:233` |
| `FABRIC_KEY_PATH` | Private key for that identity | `cmd/api/main.go:234` |
| `FABRIC_TLS_PATH` | Gateway TLS CA material | `cmd/api/main.go:235` |
| `FABRIC_CHANNEL_NAME` | Channel the transactions are submitted to | `cmd/api/main.go:237` |
| `JWT_SECRET` | When set (≥32 bytes) JWT auth is on; also the secret `POST /api/v1/auth/token` checks against | `cmd/api/main.go:252` |
| `JWT_IDENTITY_MAP` | Optional trusted subject mapping `subject:mspId[:role];...`: once set, validated claims must match one binding (subject, mspId and — if pinned — role) before any Fabric call, and token issuance refuses unbound subjects; a malformed value fails startup. Unset means no subject binding is enforced | `cmd/api/main.go:259`, `cmd/api/main.go:329`, `internal/auth/identity.go:68` |
| `GRPC_PORT` | Starts the gRPC listener; refuses to start unless `JWT_SECRET` is set | `cmd/api/main.go:265` |
| `TLS_CERT_PATH` | REST/gRPC served over TLS when **both** this and `TLS_KEY_PATH` are set; setting exactly one of the two now fails startup instead of falling back to plaintext | `cmd/api/main.go:501`, `:286` |
| `TLS_KEY_PATH` | Same pair as above; setting exactly one of the two fails startup instead of falling back to plaintext | `cmd/api/main.go:501`, `:417` |
| `PORT` | HTTP listen port, default `8080` | `cmd/api/main.go:248-251` |

The chaincode name is fixed to `playsmith-assets` at `cmd/api/main.go:236`.

Two caveats worth knowing before you deploy:

- **The Fabric identity is shared.** One `GatewayClient` is constructed at startup
  and every caller submits through it (`cmd/api/main.go:243`).
  `JWT_IDENTITY_MAP`, when set, gates which validated subjects may act at all,
  but the mapping never selects a different certificate: per-identity signing
  identity stays open, and `todo.md:77` tracks that gap.
- **TLS is all-or-nothing.** Only a complete certificate/key pair enables TLS on
  both faces; setting exactly one of the two fails startup rather than silently
  serving bearer tokens and the signing secret over plaintext. Neither one set
  serves plaintext, which is only acceptable in development.

`X-Issue-Secret` is a request header, not an environment variable: it must equal
`JWT_SECRET` for token issuance to succeed (`cmd/api/main.go:252`,
`cmd/api/main.go:324-333`).

## Indexer (`cmd/indexer`)

| Variable | Effect | Read at |
|---|---|---|
| `POSTGRES_DSN` | Required. Missing DSN aborts startup | `cmd/indexer/main.go:380` |
| `FABRIC_GATEWAY_ENDPOINT` | Fabric endpoint for the event stream | `cmd/indexer/main.go:452` |
| `FABRIC_CERT_PATH` | Identity certificate | `cmd/indexer/main.go:453` |
| `FABRIC_KEY_PATH` | Identity private key | `cmd/indexer/main.go:454` |
| `FABRIC_TLS_PATH` | Gateway TLS material | `cmd/indexer/main.go:455` |
| `FABRIC_CHANNEL_NAME` | Channel to subscribe | `cmd/indexer/main.go:456` |
| `FABRIC_MSP_ID` | MSP of the subscribing identity | `cmd/indexer/main.go:457` |

The projection schema is embedded DDL applied idempotently at startup
(`cmd/indexer/main.go:30`); there is no migration file.

## Deployment contract

`deploy/collections.json` is the only deployment input: it declares the
`playsmithPvt` collection, its two-organization read/write membership, quorum,
lifetime and endorsement policy. `deploy/README.md` covers the local
fabric-samples/test-network walkthrough.
