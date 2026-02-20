# Phase 8: Embedding & Polish (Outline)

> Will be expanded into a detailed plan before building.

<objective>
Wire everything into a deployable single binary: embed the SvelteKit build via go:embed, configure SPA fallback routing, finalize the full 14-step startup sequence, run integration tests end-to-end, and perform final quality audits.
</objective>

<features>
F23 — Single Binary + Embedded SPA (go:embed web/poller/build, SPA fallback, immutable cache for _app/)
</features>

<hdpay_reuse>
- Reference HDPay's `internal/api/handlers/spa.go` for go:embed + SPA fallback pattern
</hdpay_reuse>

<tasks_outline>
1. Build SvelteKit for production (`npm run build` in `web/poller/` → `web/poller/build/`)
2. go:embed directive in Poller's router (`//go:embed all:web/poller/build`)
3. Static file serving with SPA fallback (serve file if exists, otherwise index.html)
4. Immutable cache headers for `_app/` directory (SvelteKit hashed filenames)
5. Finalize cmd/poller/main.go startup sequence (all 14 steps from PROMPT.md):
   1. Load .env via envconfig
   2. Init slog via logging.Setup()
   3. Open SQLite + run migrations
   4. Load tiers.json (create defaults if missing)
   5. Bcrypt-hash admin password
   6. Load IP allowlist cache from DB
   7. Init price service (HDPay's PriceService)
   8. Init providers per chain (with rate limiters + circuit breakers)
   9. Init watcher orchestrator
   10. Recovery: expire active watches, re-check pending txs
   11. Setup Chi router with all middleware + handlers
   12. Embed and serve SvelteKit build
   13. Start HTTP server on POLLER_PORT
   14. Listen for SIGTERM/SIGINT → graceful shutdown
6. Integration test: start server → POST /watch → simulate tx detection → verify GET /points → POST /points/claim → verify reset
7. Constants audit: grep for hardcoded values across all Poller code, move to constants files
8. Utility dedup audit: scan for duplicated functions across internal/poller/ packages
9. Security audit: verify no private keys stored/logged, IP allowlist enforced, session cookies secure
10. Test coverage report: ensure > 70% on core packages (watcher, points, provider, pollerdb, validate)
11. Build final binary: `go build -o poller cmd/poller/main.go`
12. Smoke test: run binary with .env → health endpoint → login → create watch → verify dashboard shows data
</tasks_outline>

<research_needed>
- HDPay's spa.go implementation for go:embed + SPA fallback pattern
- Confirm SvelteKit adapter-static output structure matches go:embed expectations
</research_needed>
