import { EXPLORER_TX_URL, EXPLORER_TX_URL_TESTNET } from '$lib/constants';
import type { Chain } from '$lib/types';

/**
 * Build a block explorer URL for a transaction hash.
 * Handles SOL composite hashes (e.g. "signature:TOKEN") by stripping the suffix.
 */
export function getTxExplorerUrl(chain: string, txHash: string, network: string): string {
	const urls = network === 'testnet' ? EXPLORER_TX_URL_TESTNET : EXPLORER_TX_URL;
	const baseUrl = urls[chain as Chain];
	if (!baseUrl) return '#';

	// SOL composite hashes: "5xYz...abc:SOL" or "5xYz...abc:USDC"
	let cleanHash = txHash;
	if (chain === 'SOL' && txHash.includes(':')) {
		cleanHash = txHash.split(':')[0];
	}

	return `${baseUrl}${cleanHash}`;
}
