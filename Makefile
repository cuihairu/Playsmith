# PlaySmith build and proto generation.
#
# The gateway protobuf/gRPC bindings in internal/gatewaypb/gen are generated
# from proto/playsmith/v1/gateway.proto. They must not be edited by hand:
# `make build` / `make test` run `proto-check` first, which fails if the
# committed generated code drifts from a fresh generation (forbidden-modification
# guard), so handwritten interface changes cannot hide inside the generated
# surface.

GO        ?= go
PROTOC    ?= protoc
MODULE    := playsmith
PROTO_DIR := proto
PROTO_SRC := $(PROTO_DIR)/playsmith/v1/gateway.proto
GEN_DIR   := internal/gatewaypb/gen

# Pinned generation toolchain. Plugins install with `make proto-install`.
# protoc itself is a native binary, so `proto-doctor` verifies its version and
# says what to install. Generated files embed the protoc version in their
# header, so any other protoc would stamp and compare a different version
# string even when the protocol is unchanged.
PROTOC_VERSION              := 3.21.12
PROTOC_GEN_GO_VERSION       := v1.36.12
PROTOC_GEN_GO_GRPC_VERSION  := v1.6.2

.PHONY: build test vet proto proto-install proto-check proto-doctor clean

# Guarded entry points: generation drift fails before code compiles/tests.
build: proto-check
	$(GO) build ./...

test: proto-check
	$(GO) test ./...

vet:
	$(GO) vet ./...

# Verify protoc exists and matches PROTOC_VERSION exactly.
proto-doctor:
	@command -v $(PROTOC) >/dev/null 2>&1 || { \
		echo "ERROR: $(PROTOC) not found; install protoc $(PROTOC_VERSION)"; \
		exit 1; \
	}
	@actual=$$($(PROTOC) --version 2>/dev/null | awk '{print $$2}'); \
	if [ "$$actual" != "$(PROTOC_VERSION)" ]; then \
		echo "ERROR: protoc version mismatch: found $${actual:-unknown}, need $(PROTOC_VERSION)"; \
		echo "generated code embeds the protoc version; install protoc $(PROTOC_VERSION) or bump PROTOC_VERSION and run 'make proto'"; \
		exit 1; \
	fi

# Regenerate the gateway protobuf/gRPC bindings in place.
proto: proto-doctor
	$(PROTOC) --proto_path=. \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		$(PROTO_SRC)

# Install the pinned protoc plugins, then verify the whole toolchain.
proto-install:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	@$(MAKE) proto-doctor

# Fail if the committed generated code differs from a fresh generation.
# Only the protoc-produced files are compared; handwritten markers (doc.go,
# README.md) in the gen directory are intentionally not generated.
proto-check: proto-doctor
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	$(PROTOC) --proto_path=. \
		--go_out="$$tmp" --go_opt=module=$(MODULE) \
		--go-grpc_out="$$tmp" --go-grpc_opt=module=$(MODULE) \
		$(PROTO_SRC); \
	fail=0; \
	for f in gateway.pb.go gateway_grpc.pb.go; do \
		if ! diff -q "$$tmp/$(GEN_DIR)/$$f" "$(GEN_DIR)/$$f" >/dev/null 2>&1; then \
			echo "ERROR: $$f is out of date; run 'make proto'"; \
			diff "$$tmp/$(GEN_DIR)/$$f" "$(GEN_DIR)/$$f" | head -40; \
			fail=1; \
		fi; \
	done; \
	if [ $$fail -ne 0 ]; then exit 1; fi; \
	echo "OK: generated code is up to date"

clean:
	$(GO) clean ./...
