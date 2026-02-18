package wallet

import "errors"

var (
	ErrInvalidMnemonic = errors.New("invalid mnemonic")
	ErrDerivation      = errors.New("key derivation failed")
)
