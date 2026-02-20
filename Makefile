.PHONY: dev dev-frontend build test test-backend test-frontend check-frontend lint clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
GO := PATH=$(PATH):/usr/local/go/bin go

# Run Go backend
dev:
	$(GO) run $(LDFLAGS) ./cmd/server serve

# Run SvelteKit dev server
dev-frontend:
	cd web && npm run dev

# Build frontend then compile Go binary with embedded static files
build: build-frontend
	$(GO) build $(LDFLAGS) -o hdpay ./cmd/server

# Build frontend only
build-frontend:
	cd web && npm run build

# Run all tests (backend + frontend)
test: test-backend test-frontend

# Run Go tests
test-backend:
	$(GO) test ./... -count=1

# Run frontend unit tests (Vitest)
test-frontend:
	cd web && npx vitest run

# Run frontend type checks (svelte-check)
check-frontend:
	cd web && npm run check

# Run Go vet
lint:
	$(GO) vet ./...

# Clean build artifacts
clean:
	rm -f hdpay
	rm -rf web/build web/.svelte-kit
