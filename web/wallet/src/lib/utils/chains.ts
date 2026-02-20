import { CHAIN_COLORS } from '$lib/constants';
import type { Chain } from '$lib/types';

const CHAIN_LABELS: Record<Chain, string> = {
	BTC: 'Bitcoin',
	BSC: 'BNB Chain',
	SOL: 'Solana'
};

const EXPLORER_URLS: Record<string, Record<string, string>> = {
	mainnet: {
		BTC: 'https://mempool.space',
		BSC: 'https://bscscan.com',
		SOL: 'https://solscan.io'
	},
	testnet: {
		BTC: 'https://mempool.space/testnet',
		BSC: 'https://testnet.bscscan.com',
		SOL: 'https://solscan.io'
	}
};

export function getChainColor(chain: Chain): string {
	return CHAIN_COLORS[chain];
}

export function getChainLabel(chain: Chain): string {
	return CHAIN_LABELS[chain];
}

export function getExplorerUrl(
	chain: Chain,
	type: 'address' | 'tx',
	hash: string,
	network: string = 'mainnet'
): string {
	const base = EXPLORER_URLS[network]?.[chain] ?? EXPLORER_URLS['mainnet'][chain];

	switch (chain) {
		case 'BTC':
			return `${base}/${type === 'address' ? 'address' : 'tx'}/${hash}`;
		case 'BSC':
			return `${base}/${type === 'address' ? 'address' : 'tx'}/${hash}`;
		case 'SOL':
			return type === 'address'
				? `${base}/account/${hash}${network === 'testnet' ? '?cluster=devnet' : ''}`
				: `${base}/tx/${hash}${network === 'testnet' ? '?cluster=devnet' : ''}`;
	}
}

// getExplorerTxUrl is a convenience wrapper for transaction URLs.
export function getExplorerTxUrl(chain: Chain, txHash: string, network: string = 'mainnet'): string {
	return getExplorerUrl(chain, 'tx', txHash, network);
}
