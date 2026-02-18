import type { Chain } from '$lib/types';

// BTC bech32 address pattern (mainnet bc1, testnet tb1).
const BTC_BECH32_REGEX = /^(bc1|tb1)[a-zA-HJ-NP-Z0-9]{25,62}$/i;
// BTC legacy P2PKH / P2SH (mainnet 1/3, testnet m/n/2).
const BTC_LEGACY_REGEX = /^[13mn2][a-km-zA-HJ-NP-Z1-9]{25,34}$/;

// BSC (EVM) hex address: 0x followed by 40 hex chars.
const BSC_ADDRESS_REGEX = /^0x[0-9a-fA-F]{40}$/;

// SOL base58 address: 32-44 chars, base58 alphabet (no 0OIl).
const SOL_ADDRESS_REGEX = /^[1-9A-HJ-NP-Za-km-z]{32,44}$/;

/**
 * Validates a destination address for a given chain.
 * Returns null if valid, an error message string if invalid.
 */
export function validateAddress(chain: Chain, address: string): string | null {
	if (!address || address.trim().length === 0) {
		return 'Destination address is required';
	}

	const trimmed = address.trim();

	switch (chain) {
		case 'BTC':
			if (BTC_BECH32_REGEX.test(trimmed) || BTC_LEGACY_REGEX.test(trimmed)) {
				return null;
			}
			return 'Invalid BTC address. Expected bech32 (bc1/tb1...) or legacy format.';

		case 'BSC':
			if (BSC_ADDRESS_REGEX.test(trimmed)) {
				return null;
			}
			return 'Invalid BSC address. Expected 0x-prefixed hex (42 chars).';

		case 'SOL':
			if (SOL_ADDRESS_REGEX.test(trimmed)) {
				return null;
			}
			return 'Invalid SOL address. Expected base58 encoded (32-44 chars).';

		default:
			return `Unsupported chain: ${chain}`;
	}
}

/**
 * Returns true if the address is valid for the given chain.
 */
export function isValidDestination(chain: Chain, address: string): boolean {
	return validateAddress(chain, address) === null;
}
