# Deploy (dev)

Use fabric-samples/test-network for local Fabric. Then run API and Indexer.

- Start Fabric network: see README steps.
- Start API: `go run ./cmd/api`
- Start Indexer: `go run ./cmd/indexer`

JWT / gRPC（可选）
- Export `JWT_SECRET` (≥32 bytes) to enable JWT auth on REST.
- `POST /api/v1/auth/token` with header `X-Issue-Secret: $JWT_SECRET` to mint a bearer token.
- Export `GRPC_PORT` (e.g. `9090`) to also serve the gRPC Gateway (proto in `proto/playsmith/v1/gateway.proto`); REST and gRPC both accept `Authorization: Bearer <token>`.
