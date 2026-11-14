# Build and Test

The repository has two Go modules: the root module `playsmith` and the chaincode
module `chaincode/playsmith-assets`, which has its own `go.mod` because Fabric
deploys that directory as chaincode.

## Make targets

| Target | Does | Evidence |
|---|---|---|
| `make build` | Runs the generated-code guard, then `go build ./...` | `Makefile:29` |
| `make test` | Runs the guard, then `go test ./...` | `Makefile:32` |
| `make vet` | `go vet ./...` | `Makefile:35` |
| `make proto` | Regenerates the gateway bindings from `proto/playsmith/v1/gateway.proto` | `Makefile:52` |
| `make proto-install` | Installs the pinned plugins, then verifies the toolchain | `Makefile:59` |
| `make proto-check` | Fails if committed generated code drifts from a fresh generation | `Makefile:67` |
| `make proto-doctor` | Checks `protoc` exists and matches the pinned version exactly | `Makefile:39` |
| `make clean` | `go clean ./...` | `Makefile:85` |

The guard is the important part. `make build` and `make test` both depend on
`proto-check`, which regenerates the two protobuf files into a temporary
directory and byte-compares them against
`internal/gatewaypb/gen`. Hand-editing a generated file therefore fails before any
code compiles, so an interface change cannot hide inside the generated surface.

The toolchain is pinned because the generated files embed the protoc version in
their header — a different protoc would stamp a different string and report drift
even when the protocol never changed (`Makefile:17-24`):

| Component | Pinned version |
|---|---|
| `protoc` | `3.21.12` |
| `protoc-gen-go` | `v1.36.12` |
| `protoc-gen-go-grpc` | `v1.6.2` |

`make proto-check PROTOC_VERSION=9.9.9` reports a version mismatch rather than a
false "generated code is out of date".

## Local Fabric quick start

Using fabric-samples/test-network:

1. Bring the network up with one channel and a CA:
   `./network.sh up createChannel -c playsmith -ca`
2. Deploy the chaincode:
   `./network.sh deployCC -c playsmith -ccn playsmith-assets -ccl go -ccp ../../Playsmith/chaincode/playsmith-assets`
3. Run the app layer: `cmd/api` (set `FABRIC_GATEWAY_ENDPOINT` and friends for a
   real Fabric backend; unset, it runs `NoopClient`), then `cmd/indexer` with
   `POSTGRES_DSN`.
4. Export `JWT_SECRET` (≥32 bytes) to turn on auth, and `GRPC_PORT` for the
   gRPC face.

See `deploy/README.md` in the repository for the shorter local variant and
[the configuration page](/configuration) for every variable.

## Deployment

`docs/` is a VitePress site published as a GitHub Pages project page under
`/Playsmith/`. Pushing to `docs/**` triggers `.github/workflows/pages.yml`, which
installs with pnpm, builds `@playsmith/docs` and uploads
`docs/.vitepress/dist`.
