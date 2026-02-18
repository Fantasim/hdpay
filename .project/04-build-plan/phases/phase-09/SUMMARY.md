# Phase 9 Summary: SOL Transaction Engine

## Completed: 2026-02-18

## What Was Built
- Raw Solana binary transaction serialization layer (compact-u16, message headers, compiled instructions, signing) — zero external SDK
- SystemProgram.Transfer instruction builder (native SOL transfers)
- SPL Token.Transfer instruction builder (USDC/USDT token transfers)
- CreateAssociatedTokenAccount instruction builder (for destination ATA creation)
- SOL private key derivation in KeyService (SLIP-10 ed25519 path m/44'/501'/N'/0')
- SOL JSON-RPC client with round-robin URL selection (getLatestBlockhash, sendTransaction, getSignatureStatuses, getAccountInfo, getBalance)
- Confirmation polling with configurable timeout and commitment level
- SOLConsolidationService: sequential per-address native SOL sweep + SPL token sweep with auto-ATA creation
- Preview methods for both native and token sweeps (dry-run cost/balance calculation)
- 29 new tests covering serialization, service logic, key derivation, and edge cases

## Files Created/Modified
- `internal/tx/sol_serialize.go` — **NEW** Core Solana binary serialization: types, compact-u16 encoding, instruction builders, message compilation, transaction serialization, ed25519 signing (~370 lines)
- `internal/tx/sol_tx.go` — **NEW** SOLRPCClient interface, DefaultSOLRPCClient (JSON-RPC), WaitForSOLConfirmation, SOLConsolidationService with Preview/Execute for native + token sweeps (~1010 lines)
- `internal/tx/sol_serialize_test.go` — **NEW** 13 tests for serialization layer (compact-u16, instruction builders, message compilation, signing)
- `internal/tx/sol_tx_test.go` — **NEW** 14 tests for RPC client, consolidation service, confirmation polling (with mock SOLRPCClient)
- `internal/tx/key_service.go` — Added `DeriveSOLPrivateKey` method using SLIP-10 from raw BIP-39 seed
- `internal/tx/key_service_test.go` — Added 2 SOL key derivation tests (known vector + multiple indices)
- `internal/config/constants.go` — Added SOL transaction constants (lamports, fees, TX size, confirmation timing, program IDs)
- `internal/config/errors.go` — Added 5 SOL sentinel errors + 5 error codes
- `internal/models/types.go` — Added `SOLSendPreview`, `SOLSendResult`, `SOLTxResult` structs

## Decisions Made
- **Sequential per-address sends** (not multi-signer batch): Multi-signer consolidation is limited to ~7 addresses per TX due to the 1232-byte TX limit and 64-byte signatures. Sequential sends (like BSC) are simpler, more reliable, and have no practical throughput concern for the use case.
- **Zero external Solana SDK**: All serialization, instruction encoding, and RPC calls built from scratch using only `crypto/ed25519`, `encoding/binary`, and `mr-tron/base58`. This avoids the heavy `gagliardetto/solana-go` dependency.
- **Reuse scanner.DeriveATA**: ATA derivation for SPL token sweeps reuses the existing PDA derivation from `internal/scanner/sol_ata.go` (no import cycle since tx already imports scanner).
- **Nil-safe database recording**: `recordSOLTransaction` handles nil database gracefully (logs warning, continues) to support testing without DB dependency.
- **Legacy (non-versioned) transactions**: Uses legacy message format (no version prefix) for maximum compatibility with all Solana validators.

## Deviations from Plan
- Plan mentioned `DeriveSOLAddress` helper on KeyService — instead reused existing `wallet.DeriveSOLAddress` for address verification in tests
- Plan outlined separate `BuildNativeSOLTx` and `BuildSPLTokenTx` top-level functions — instead integrated directly into consolidation service methods since they were only used there
- Added `BuildAndSerializeTransaction` convenience function (not in plan) for clean single-call TX building

## Issues Encountered
- **Nil pointer dereference in tests**: When `SOLConsolidationService` was created with nil database (for unit tests), calling `recordSOLTransaction` caused SIGSEGV. Fixed by adding nil check at top of the method.

## Notes for Next Phase
- Phase 10 (Send Interface) will need to wire `SOLConsolidationService` into the send API handlers alongside BTC and BSC services
- The `SOLRPCClient` interface is ready for dependency injection in API handler tests
- Preview methods return detailed cost breakdowns that the frontend can display directly
- Token sweep handles ATA creation transparently — the frontend doesn't need to know about ATAs
