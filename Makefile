.PHONY: dev dev-wallet-frontend dev-poller dev-poller-frontend \
       build build-all build-wallet build-poller build-verify \
       build-wallet-frontend build-poller-frontend \
       test test-backend test-wallet test-poller test-wallet-frontend \
       check-wallet-frontend lint clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
GO := PATH=$(PATH):/usr/local/go/bin go

# ── Build All ────────────────────────────────────────

# Build all binaries (wallet + poller + verify)
build-all: build-wallet build-poller build-verify

# ── Wallet ────────────────────────────────────────────

# Run Wallet Go backend
dev:
	$(GO) run $(LDFLAGS) ./cmd/wallet serve

# Run Wallet SvelteKit dev server
dev-wallet-frontend:
	cd web/wallet && npm run dev

# Build Wallet frontend then compile Go binary with embedded static files
build-wallet: build-wallet-frontend
	$(GO) build $(LDFLAGS) -o bin/wallet ./cmd/wallet

# Build Wallet frontend only
build-wallet-frontend:
	cd web/wallet && npm run build

# ── Poller ─────────────────────────────────────────────

# Run Poller Go backend
dev-poller:
	$(GO) run $(LDFLAGS) ./cmd/poller

# Run Poller SvelteKit dev server
dev-poller-frontend:
	cd web/poller && npm run dev

# Build Poller frontend only
build-poller-frontend:
	cd web/poller && npm run build

# Build Poller binary (frontend + Go with embedded SPA)
build-poller: build-poller-frontend
	$(GO) build $(LDFLAGS) -o bin/poller ./cmd/poller

# ── Verify ─────────────────────────────────────────────

# Build Verify utility
build-verify:
	$(GO) build $(LDFLAGS) -o bin/verify ./cmd/verify

# ── Tests ──────────────────────────────────────────────

# Run all tests (backend + wallet frontend)
test: test-backend test-wallet-frontend

# Run all Go tests
test-backend:
	$(GO) test ./... -count=1

# Run Wallet Go tests only
test-wallet:
	$(GO) test ./internal/wallet/... -count=1 -v

# Run Poller Go tests only
test-poller:
	$(GO) test ./internal/poller/... -count=1 -v

# Run Wallet frontend unit tests (Vitest)
test-wallet-frontend:
	cd web/wallet && npx vitest run

# Run Wallet frontend type checks (svelte-check)
check-wallet-frontend:
	cd web/wallet && npm run check

# ── Quality ────────────────────────────────────────────

# Run Go vet
lint:
	$(GO) vet ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f hdpay
	rm -rf web/wallet/build web/wallet/.svelte-kit web/poller/build web/poller/.svelte-kit
