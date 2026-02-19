package tx

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"

	"github.com/Fantasim/hdpay/internal/config"
	"github.com/Fantasim/hdpay/internal/db"
	"github.com/Fantasim/hdpay/internal/models"
)

// SigningUTXO extends a UTXO with the data needed for signing.
type SigningUTXO struct {
	models.UTXO
	PKScript []byte
	PrivKey  *btcec.PrivateKey
}

// BTCBuildParams contains the parameters for building a BTC consolidation transaction.
type BTCBuildParams struct {
	UTXOs       []models.UTXO
	DestAddress string
	FeeRate     int64 // sat/vB
	NetParams   *chaincfg.Params
}

// BTCBuiltTx contains the result of building (but not signing) a transaction.
type BTCBuiltTx struct {
	Tx             *wire.MsgTx
	UTXOs          []models.UTXO
	TotalInputSats int64
	OutputSats     int64
	FeeSats        int64
	EstimatedVsize int
}

// EstimateBTCVsize returns the estimated vsize of a P2WPKH-only transaction.
func EstimateBTCVsize(numInputs, numOutputs int) int {
	weight := config.BTCTxOverheadWU +
		numInputs*(config.BTCP2WPKHInputNonWitWU+config.BTCP2WPKHInputWitWU) +
		numOutputs*config.BTCP2WPKHOutputWU
	// ceil(weight / 4)
	return (weight + 3) / 4
}

// BuildBTCConsolidationTx builds an unsigned multi-input consolidation transaction.
// All UTXOs are spent to a single destination address.
func BuildBTCConsolidationTx(params BTCBuildParams) (*BTCBuiltTx, error) {
	if len(params.UTXOs) == 0 {
		return nil, fmt.Errorf("%w: no UTXOs provided", config.ErrInsufficientUTXO)
	}

	if len(params.UTXOs) > config.BTCMaxInputsPerTx {
		return nil, fmt.Errorf("%w: %d inputs exceeds maximum %d",
			config.ErrTxTooLarge, len(params.UTXOs), config.BTCMaxInputsPerTx)
	}

	slog.Info("building BTC consolidation transaction",
		"inputCount", len(params.UTXOs),
		"destAddress", params.DestAddress,
		"feeRate", params.FeeRate,
	)

	// Validate and decode destination address.
	destAddr, err := btcutil.DecodeAddress(params.DestAddress, params.NetParams)
	if err != nil {
		return nil, fmt.Errorf("decode destination address %q: %w", params.DestAddress, err)
	}

	destScript, err := txscript.PayToAddrScript(destAddr)
	if err != nil {
		return nil, fmt.Errorf("create destination script: %w", err)
	}

	// Calculate totals.
	var totalInputSats int64
	for _, u := range params.UTXOs {
		totalInputSats += u.Value
	}

	estimatedVsize := EstimateBTCVsize(len(params.UTXOs), 1)
	feeSats := params.FeeRate * int64(estimatedVsize)
	outputSats := totalInputSats - feeSats

	slog.Info("BTC TX fee calculation",
		"totalInputSats", totalInputSats,
		"estimatedVsize", estimatedVsize,
		"feeRate", params.FeeRate,
		"feeSats", feeSats,
		"outputSats", outputSats,
	)

	if outputSats <= 0 {
		return nil, fmt.Errorf("%w: total %d sats, fee %d sats", config.ErrInsufficientUTXO, totalInputSats, feeSats)
	}

	if outputSats < int64(config.BTCDustThresholdSats) {
		return nil, fmt.Errorf("%w: output %d sats below dust threshold %d",
			config.ErrDustOutput, outputSats, config.BTCDustThresholdSats)
	}

	// Check weight limit.
	estimatedWeight := config.BTCTxOverheadWU +
		len(params.UTXOs)*(config.BTCP2WPKHInputNonWitWU+config.BTCP2WPKHInputWitWU) +
		1*config.BTCP2WPKHOutputWU
	if estimatedWeight > config.BTCMaxTxWeight {
		return nil, fmt.Errorf("%w: estimated weight %d exceeds max %d",
			config.ErrTxTooLarge, estimatedWeight, config.BTCMaxTxWeight)
	}

	// Build the transaction.
	msgTx := wire.NewMsgTx(wire.TxVersion)

	for _, u := range params.UTXOs {
		hash, err := chainhash.NewHashFromStr(u.TxID)
		if err != nil {
			return nil, fmt.Errorf("parse UTXO txid %q: %w", u.TxID, err)
		}
		outPoint := wire.NewOutPoint(hash, u.Vout)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		txIn.Sequence = wire.MaxTxInSequenceNum
		msgTx.AddTxIn(txIn)
	}

	msgTx.AddTxOut(wire.NewTxOut(outputSats, destScript))

	slog.Debug("BTC TX built (unsigned)",
		"inputs", len(msgTx.TxIn),
		"outputs", len(msgTx.TxOut),
		"outputSats", outputSats,
	)

	return &BTCBuiltTx{
		Tx:             msgTx,
		UTXOs:          params.UTXOs,
		TotalInputSats: totalInputSats,
		OutputSats:     outputSats,
		FeeSats:        feeSats,
		EstimatedVsize: estimatedVsize,
	}, nil
}

// SignBTCTx signs each input with P2WPKH witness data.
// Uses MultiPrevOutFetcher for correct multi-input signing.
// Each private key is zeroed after signing its input.
func SignBTCTx(msgTx *wire.MsgTx, signingUTXOs []SigningUTXO) error {
	if len(msgTx.TxIn) != len(signingUTXOs) {
		return fmt.Errorf("input count mismatch: tx has %d inputs, got %d signing UTXOs",
			len(msgTx.TxIn), len(signingUTXOs))
	}

	slog.Info("signing BTC transaction", "inputCount", len(signingUTXOs))

	// Build MultiPrevOutFetcher — maps each outpoint to its previous TxOut.
	prevOutFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for i, su := range signingUTXOs {
		hash, err := chainhash.NewHashFromStr(su.TxID)
		if err != nil {
			return fmt.Errorf("parse signing UTXO txid at index %d: %w", i, err)
		}
		op := wire.OutPoint{Hash: *hash, Index: su.Vout}
		prevOutFetcher.AddPrevOut(op, &wire.TxOut{
			Value:    su.Value,
			PkScript: su.PKScript,
		})
	}

	// Compute sighash midstate ONCE for all inputs (BIP-143 optimization).
	sigHashes := txscript.NewTxSigHashes(msgTx, prevOutFetcher)

	// Sign each input.
	for i, su := range signingUTXOs {
		witness, err := txscript.WitnessSignature(
			msgTx,
			sigHashes,
			i,
			su.Value,
			su.PKScript,
			txscript.SigHashAll,
			su.PrivKey,
			true, // compressed pubkey
		)
		if err != nil {
			return fmt.Errorf("sign input %d (address %s, index %d): %w",
				i, su.Address, su.AddressIndex, err)
		}

		msgTx.TxIn[i].Witness = witness
		// SignatureScript stays nil for native SegWit P2WPKH.

		// Zero the private key after signing this input.
		su.PrivKey.Zero()

		slog.Debug("BTC TX input signed",
			"inputIndex", i,
			"addressIndex", su.AddressIndex,
			"value", su.Value,
		)
	}

	slog.Info("BTC transaction signed", "inputCount", len(signingUTXOs))
	return nil
}

// SerializeBTCTx serializes a signed transaction to hex.
func SerializeBTCTx(msgTx *wire.MsgTx) (string, error) {
	var buf bytes.Buffer
	if err := msgTx.Serialize(&buf); err != nil {
		return "", fmt.Errorf("serialize BTC transaction: %w", err)
	}

	rawHex := hex.EncodeToString(buf.Bytes())
	slog.Debug("BTC TX serialized", "hexLength", len(rawHex))
	return rawHex, nil
}

// PKScriptFromAddress reconstructs the pkScript for a BTC address.
// This is needed because Esplora UTXO endpoints don't return scriptPubKey.
func PKScriptFromAddress(address string, netParams *chaincfg.Params) ([]byte, error) {
	decoded, err := btcutil.DecodeAddress(address, netParams)
	if err != nil {
		return nil, fmt.Errorf("decode address %q: %w", address, err)
	}
	pkScript, err := txscript.PayToAddrScript(decoded)
	if err != nil {
		return nil, fmt.Errorf("create pkScript for %q: %w", address, err)
	}
	return pkScript, nil
}

// BTCConsolidationService orchestrates the full BTC consolidation flow:
// fetch UTXOs → estimate fee → build tx → derive keys → sign → broadcast → confirm → record.
type BTCConsolidationService struct {
	keyService       *KeyService
	utxoFetcher      *BTCUTXOFetcher
	feeEstimator     *BTCFeeEstimator
	broadcaster      Broadcaster
	database         *db.DB
	netParams        *chaincfg.Params
	httpClient       *http.Client
	confirmationURLs []string // Esplora-compatible base URLs for TX status polling
}

// NewBTCConsolidationService creates the consolidation orchestrator.
func NewBTCConsolidationService(
	keyService *KeyService,
	utxoFetcher *BTCUTXOFetcher,
	feeEstimator *BTCFeeEstimator,
	broadcaster Broadcaster,
	database *db.DB,
	netParams *chaincfg.Params,
	httpClient *http.Client,
	confirmationURLs []string,
) *BTCConsolidationService {
	slog.Info("BTC consolidation service created",
		"network", netParams.Name,
		"confirmationURLs", confirmationURLs,
	)
	return &BTCConsolidationService{
		keyService:       keyService,
		utxoFetcher:      utxoFetcher,
		feeEstimator:     feeEstimator,
		broadcaster:      broadcaster,
		database:         database,
		netParams:        netParams,
		httpClient:       httpClient,
		confirmationURLs: confirmationURLs,
	}
}

// Preview performs a dry run of the consolidation: fetches UTXOs, estimates fee,
// and returns the expected transaction details without signing or broadcasting.
func (s *BTCConsolidationService) Preview(ctx context.Context, addresses []models.Address, destAddr string, feeRate int64) (*models.SendPreview, error) {
	slog.Info("BTC consolidation preview",
		"addressCount", len(addresses),
		"destAddress", destAddr,
		"requestedFeeRate", feeRate,
	)

	utxos, err := s.utxoFetcher.FetchAllUTXOs(ctx, addresses)
	if err != nil {
		return nil, fmt.Errorf("fetch UTXOs for preview: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("%w: no confirmed UTXOs found", config.ErrInsufficientUTXO)
	}

	// Use provided feeRate or estimate.
	if feeRate <= 0 {
		estimate, err := s.feeEstimator.EstimateFee(ctx)
		if err != nil {
			return nil, fmt.Errorf("estimate fee for preview: %w", err)
		}
		feeRate = DefaultFeeRate(estimate)
	}

	built, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: destAddr,
		FeeRate:     feeRate,
		NetParams:   s.netParams,
	})
	if err != nil {
		return nil, fmt.Errorf("build preview TX: %w", err)
	}

	preview := &models.SendPreview{
		Chain:          models.ChainBTC,
		InputCount:     len(built.UTXOs),
		TotalInputSats: built.TotalInputSats,
		OutputSats:     built.OutputSats,
		FeeSats:        built.FeeSats,
		FeeRate:        feeRate,
		EstimatedVsize: built.EstimatedVsize,
		DestAddress:    destAddr,
	}

	slog.Info("BTC consolidation preview complete",
		"inputCount", preview.InputCount,
		"totalInputSats", preview.TotalInputSats,
		"outputSats", preview.OutputSats,
		"feeSats", preview.FeeSats,
	)

	return preview, nil
}

// Execute performs the full consolidation: fetch UTXOs → validate → build → sign → broadcast → confirm → record.
// If expectedInputCount > 0, validates that re-fetched UTXOs haven't diverged significantly from preview.
func (s *BTCConsolidationService) Execute(ctx context.Context, addresses []models.Address, destAddr string, feeRate int64, sweepID string, expectedInputCount int, expectedTotalSats int64) (*models.SendResult, error) {
	slog.Info("BTC consolidation execute",
		"addressCount", len(addresses),
		"destAddress", destAddr,
		"requestedFeeRate", feeRate,
		"sweepID", sweepID,
		"expectedInputCount", expectedInputCount,
		"expectedTotalSats", expectedTotalSats,
	)
	start := time.Now()

	// Create tx_state row for this consolidated TX.
	txStateID := GenerateTxStateID()
	txState := db.TxStateRow{
		ID:           txStateID,
		SweepID:      sweepID,
		Chain:        string(models.ChainBTC),
		Token:        string(models.TokenNative),
		AddressIndex: 0, // BTC consolidation is multi-input, no single index
		FromAddress:  "consolidated",
		ToAddress:    destAddr,
		Amount:       "0", // Updated after building
		Status:       config.TxStatePending,
	}
	if err := s.database.CreateTxState(txState); err != nil {
		slog.Error("failed to create BTC tx_state", "error", err)
		// Non-blocking: continue even if tx_state write fails
	}

	// 1. Fetch UTXOs.
	utxos, err := s.utxoFetcher.FetchAllUTXOs(ctx, addresses)
	if err != nil {
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("fetch UTXOs: %s", err))
		return nil, fmt.Errorf("fetch UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		s.updateTxState(txStateID, config.TxStateFailed, "", "no confirmed UTXOs found")
		return nil, fmt.Errorf("%w: no confirmed UTXOs found", config.ErrInsufficientUTXO)
	}

	// 1b. Validate UTXOs against preview expectations (if provided).
	if expectedInputCount > 0 {
		if err := ValidateUTXOsAgainstPreview(utxos, expectedInputCount, expectedTotalSats); err != nil {
			s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("UTXO validation: %s", err))
			return nil, err
		}
	}

	// 2. Estimate fee if not provided.
	if feeRate <= 0 {
		estimate, err := s.feeEstimator.EstimateFee(ctx)
		if err != nil {
			s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("estimate fee: %s", err))
			return nil, fmt.Errorf("estimate fee: %w", err)
		}
		feeRate = DefaultFeeRate(estimate)
	}

	// 3. Build unsigned transaction.
	built, err := BuildBTCConsolidationTx(BTCBuildParams{
		UTXOs:       utxos,
		DestAddress: destAddr,
		FeeRate:     feeRate,
		NetParams:   s.netParams,
	})
	if err != nil {
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("build TX: %s", err))
		return nil, fmt.Errorf("build TX: %w", err)
	}

	// Update tx_state with actual amount.
	s.updateTxState(txStateID, config.TxStatePending, "", "")
	if err := s.database.UpdateTxStatus(txStateID, config.TxStatePending, "", ""); err != nil {
		slog.Error("failed to update BTC tx_state amount", "error", err)
	}

	// 4. Derive keys and prepare signing UTXOs.
	signingUTXOs, err := s.prepareSigningUTXOs(ctx, built.UTXOs)
	if err != nil {
		// Zero any keys already derived on error.
		for _, su := range signingUTXOs {
			if su.PrivKey != nil {
				su.PrivKey.Zero()
			}
		}
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("prepare signing: %s", err))
		return nil, fmt.Errorf("prepare signing UTXOs: %w", err)
	}

	// 5. Sign the transaction.
	if err := SignBTCTx(built.Tx, signingUTXOs); err != nil {
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("sign TX: %s", err))
		return nil, fmt.Errorf("sign TX: %w", err)
	}

	// 6. Serialize to hex.
	rawHex, err := SerializeBTCTx(built.Tx)
	if err != nil {
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("serialize TX: %s", err))
		return nil, fmt.Errorf("serialize TX: %w", err)
	}

	slog.Info("BTC TX built and signed, broadcasting",
		"hexLength", len(rawHex),
		"inputCount", len(built.UTXOs),
		"outputSats", built.OutputSats,
		"feeSats", built.FeeSats,
		"txStateID", txStateID,
	)

	// 7. Update to broadcasting.
	s.updateTxState(txStateID, config.TxStateBroadcasting, "", "")

	// 8. Broadcast.
	txHash, err := s.broadcaster.Broadcast(ctx, rawHex)
	if err != nil {
		s.updateTxState(txStateID, config.TxStateFailed, "", fmt.Sprintf("broadcast: %s", err))
		return nil, fmt.Errorf("broadcast TX: %w", err)
	}

	slog.Info("BTC TX broadcast successful",
		"txHash", txHash,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	// 9. Update to confirming with txHash.
	s.updateTxState(txStateID, config.TxStateConfirming, txHash, "")

	// 10. Record in transactions table.
	if err := s.recordTransaction(ctx, txHash, built, destAddr); err != nil {
		slog.Error("failed to record BTC transaction in DB",
			"txHash", txHash,
			"error", err,
		)
	}

	// 11. Wait for confirmation (best-effort — timeout is not a failure).
	if err := WaitForBTCConfirmation(ctx, s.httpClient, s.confirmationURLs, txHash); err != nil {
		slog.Warn("BTC confirmation polling timed out or failed",
			"txHash", txHash,
			"error", err,
		)
		s.updateTxState(txStateID, config.TxStateUncertain, txHash, fmt.Sprintf("confirmation: %s", err))
	} else {
		slog.Info("BTC transaction confirmed", "txHash", txHash)
		s.updateTxState(txStateID, config.TxStateConfirmed, txHash, "")
	}

	return &models.SendResult{
		TxHash: txHash,
		Chain:  models.ChainBTC,
	}, nil
}

// updateTxState is a non-blocking helper that logs errors but doesn't propagate them.
func (s *BTCConsolidationService) updateTxState(id, status, txHash, txError string) {
	if err := s.database.UpdateTxStatus(id, status, txHash, txError); err != nil {
		slog.Error("failed to update BTC tx_state",
			"id", id,
			"status", status,
			"error", err,
		)
	}
}

// prepareSigningUTXOs derives private keys and reconstructs pkScripts for each UTXO.
func (s *BTCConsolidationService) prepareSigningUTXOs(ctx context.Context, utxos []models.UTXO) ([]SigningUTXO, error) {
	signingUTXOs := make([]SigningUTXO, 0, len(utxos))

	for _, u := range utxos {
		if err := ctx.Err(); err != nil {
			return signingUTXOs, fmt.Errorf("context cancelled during key derivation: %w", err)
		}

		pkScript, err := PKScriptFromAddress(u.Address, s.netParams)
		if err != nil {
			return signingUTXOs, fmt.Errorf("pkScript for address %s (index %d): %w", u.Address, u.AddressIndex, err)
		}

		privKey, err := s.keyService.DeriveBTCPrivateKey(ctx, uint32(u.AddressIndex))
		if err != nil {
			return signingUTXOs, fmt.Errorf("derive key for index %d: %w", u.AddressIndex, err)
		}

		signingUTXOs = append(signingUTXOs, SigningUTXO{
			UTXO:     u,
			PKScript: pkScript,
			PrivKey:  privKey,
		})
	}

	return signingUTXOs, nil
}

// recordTransaction stores the consolidation TX in the transactions table.
func (s *BTCConsolidationService) recordTransaction(_ context.Context, txHash string, built *BTCBuiltTx, destAddr string) error {
	// Record one transaction entry per input address for history tracking.
	for _, u := range built.UTXOs {
		txRecord := models.Transaction{
			Chain:        models.ChainBTC,
			AddressIndex: u.AddressIndex,
			TxHash:       txHash,
			Direction:    "send",
			Token:        models.TokenNative,
			Amount:       strconv.FormatInt(u.Value, 10),
			FromAddress:  u.Address,
			ToAddress:    destAddr,
			Status:       "pending",
		}

		if _, err := s.database.InsertTransaction(txRecord); err != nil {
			slog.Error("failed to record transaction for address",
				"addressIndex", u.AddressIndex,
				"txHash", txHash,
				"error", err,
			)
			// Continue recording other inputs.
		}
	}

	return nil
}

// --- BTC UTXO Re-Validation ---

// ValidateUTXOsAgainstPreview compares re-fetched UTXOs at execute time against the preview expectations.
// Returns an error if the UTXO set diverged significantly (UTXOs spent externally between preview and execute).
func ValidateUTXOsAgainstPreview(utxos []models.UTXO, expectedCount int, expectedTotalSats int64) error {
	actualCount := len(utxos)
	var actualTotal int64
	for _, u := range utxos {
		actualTotal += u.Value
	}

	slog.Info("BTC UTXO re-validation",
		"expectedCount", expectedCount,
		"actualCount", actualCount,
		"expectedTotalSats", expectedTotalSats,
		"actualTotalSats", actualTotal,
	)

	// Check count divergence.
	if expectedCount > 0 {
		countDrop := 1.0 - float64(actualCount)/float64(expectedCount)
		if countDrop > config.BTCUTXOCountDivergenceThreshold {
			slog.Error("BTC UTXO count diverged beyond threshold",
				"expectedCount", expectedCount,
				"actualCount", actualCount,
				"dropPercent", countDrop*100,
				"threshold", config.BTCUTXOCountDivergenceThreshold*100,
			)
			return fmt.Errorf("%w: expected %d UTXOs but found %d (%.0f%% drop, threshold %.0f%%)",
				config.ErrUTXODiverged, expectedCount, actualCount, countDrop*100, config.BTCUTXOCountDivergenceThreshold*100)
		}
	}

	// Check value divergence.
	if expectedTotalSats > 0 {
		valueDrop := 1.0 - float64(actualTotal)/float64(expectedTotalSats)
		if valueDrop > config.BTCUTXOValueDivergenceThreshold {
			slog.Error("BTC UTXO total value diverged beyond threshold",
				"expectedTotalSats", expectedTotalSats,
				"actualTotalSats", actualTotal,
				"dropPercent", valueDrop*100,
				"threshold", config.BTCUTXOValueDivergenceThreshold*100,
			)
			return fmt.Errorf("%w: expected %d sats but found %d sats (%.0f%% drop, threshold %.0f%%)",
				config.ErrUTXODiverged, expectedTotalSats, actualTotal, valueDrop*100, config.BTCUTXOValueDivergenceThreshold*100)
		}
	}

	slog.Info("BTC UTXO re-validation passed")
	return nil
}

// --- BTC Confirmation Polling ---

// btcTxStatus is the JSON response from Esplora /tx/{txid}/status endpoint.
type btcTxStatus struct {
	Confirmed   bool  `json:"confirmed"`
	BlockHeight int64 `json:"block_height"`
	BlockHash   string `json:"block_hash"`
	BlockTime   int64 `json:"block_time"`
}

// WaitForBTCConfirmation polls Esplora-compatible APIs for BTC transaction confirmation.
// Returns nil if confirmed, ErrBTCConfirmationTimeout if timeout is reached.
// This is best-effort — a timeout does NOT mean the TX failed.
func WaitForBTCConfirmation(ctx context.Context, client *http.Client, providerURLs []string, txHash string) error {
	if len(providerURLs) == 0 {
		slog.Warn("BTC confirmation polling skipped: no provider URLs configured")
		return nil
	}

	slog.Info("waiting for BTC confirmation",
		"txHash", txHash,
		"providerCount", len(providerURLs),
		"timeout", config.BTCConfirmationTimeout,
	)

	pollCtx, cancel := context.WithTimeout(ctx, config.BTCConfirmationTimeout)
	defer cancel()

	providerIdx := 0
	for {
		baseURL := providerURLs[providerIdx%len(providerURLs)]
		providerIdx++

		url := baseURL + fmt.Sprintf(config.BTCTxStatusPath, txHash)

		confirmed, err := pollBTCTxStatus(pollCtx, client, url)
		if err != nil {
			slog.Warn("BTC confirmation poll error",
				"txHash", txHash,
				"provider", baseURL,
				"error", err,
			)
			// Transient error — try next provider on next iteration
		} else if confirmed {
			slog.Info("BTC transaction confirmed via polling",
				"txHash", txHash,
				"provider", baseURL,
			)
			return nil
		}

		select {
		case <-pollCtx.Done():
			return fmt.Errorf("%w: tx %s", config.ErrBTCConfirmationTimeout, txHash)
		case <-time.After(config.BTCConfirmationPollInterval):
			slog.Debug("BTC confirmation not ready, polling again", "txHash", txHash)
		}
	}
}

// pollBTCTxStatus makes a single request to check BTC TX confirmation status.
func pollBTCTxStatus(ctx context.Context, client *http.Client, url string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("create status request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// TX not yet in mempool — not an error, just not found yet
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var status btcTxStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, fmt.Errorf("decode status response: %w", err)
	}

	return status.Confirmed, nil
}
