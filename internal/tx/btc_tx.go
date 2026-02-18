package tx

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
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
// fetch UTXOs → estimate fee → build tx → derive keys → sign → broadcast → record.
type BTCConsolidationService struct {
	keyService   *KeyService
	utxoFetcher  *BTCUTXOFetcher
	feeEstimator *BTCFeeEstimator
	broadcaster  Broadcaster
	database     *db.DB
	netParams    *chaincfg.Params
}

// NewBTCConsolidationService creates the consolidation orchestrator.
func NewBTCConsolidationService(
	keyService *KeyService,
	utxoFetcher *BTCUTXOFetcher,
	feeEstimator *BTCFeeEstimator,
	broadcaster Broadcaster,
	database *db.DB,
	netParams *chaincfg.Params,
) *BTCConsolidationService {
	slog.Info("BTC consolidation service created", "network", netParams.Name)
	return &BTCConsolidationService{
		keyService:   keyService,
		utxoFetcher:  utxoFetcher,
		feeEstimator: feeEstimator,
		broadcaster:  broadcaster,
		database:     database,
		netParams:    netParams,
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

// Execute performs the full consolidation: fetch UTXOs → build → sign → broadcast → record.
func (s *BTCConsolidationService) Execute(ctx context.Context, addresses []models.Address, destAddr string, feeRate int64) (*models.SendResult, error) {
	slog.Info("BTC consolidation execute",
		"addressCount", len(addresses),
		"destAddress", destAddr,
		"requestedFeeRate", feeRate,
	)
	start := time.Now()

	// 1. Fetch UTXOs.
	utxos, err := s.utxoFetcher.FetchAllUTXOs(ctx, addresses)
	if err != nil {
		return nil, fmt.Errorf("fetch UTXOs: %w", err)
	}

	if len(utxos) == 0 {
		return nil, fmt.Errorf("%w: no confirmed UTXOs found", config.ErrInsufficientUTXO)
	}

	// 2. Estimate fee if not provided.
	if feeRate <= 0 {
		estimate, err := s.feeEstimator.EstimateFee(ctx)
		if err != nil {
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
		return nil, fmt.Errorf("build TX: %w", err)
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
		return nil, fmt.Errorf("prepare signing UTXOs: %w", err)
	}

	// 5. Sign the transaction.
	if err := SignBTCTx(built.Tx, signingUTXOs); err != nil {
		return nil, fmt.Errorf("sign TX: %w", err)
	}

	// 6. Serialize to hex.
	rawHex, err := SerializeBTCTx(built.Tx)
	if err != nil {
		return nil, fmt.Errorf("serialize TX: %w", err)
	}

	slog.Info("BTC TX built and signed, broadcasting",
		"hexLength", len(rawHex),
		"inputCount", len(built.UTXOs),
		"outputSats", built.OutputSats,
		"feeSats", built.FeeSats,
	)

	// 7. Broadcast.
	txHash, err := s.broadcaster.Broadcast(ctx, rawHex)
	if err != nil {
		return nil, fmt.Errorf("broadcast TX: %w", err)
	}

	slog.Info("BTC TX broadcast successful",
		"txHash", txHash,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	// 8. Record in database.
	if err := s.recordTransaction(ctx, txHash, built, destAddr); err != nil {
		// Broadcast succeeded — log error but don't fail.
		slog.Error("failed to record BTC transaction in DB",
			"txHash", txHash,
			"error", err,
		)
	}

	return &models.SendResult{
		TxHash: txHash,
		Chain:  models.ChainBTC,
	}, nil
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
