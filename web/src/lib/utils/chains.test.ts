import { describe, it, expect } from 'vitest';
import { getChainColor, getChainLabel, getExplorerUrl, getExplorerTxUrl } from './chains';

describe('getChainColor', () => {
	it('returns correct color for BTC', () => {
		expect(getChainColor('BTC')).toBe('#f7931a');
	});

	it('returns correct color for BSC', () => {
		expect(getChainColor('BSC')).toBe('#F0B90B');
	});

	it('returns correct color for SOL', () => {
		expect(getChainColor('SOL')).toBe('#9945FF');
	});
});

describe('getChainLabel', () => {
	it('returns Bitcoin for BTC', () => {
		expect(getChainLabel('BTC')).toBe('Bitcoin');
	});

	it('returns BNB Chain for BSC', () => {
		expect(getChainLabel('BSC')).toBe('BNB Chain');
	});

	it('returns Solana for SOL', () => {
		expect(getChainLabel('SOL')).toBe('Solana');
	});
});

describe('getExplorerUrl', () => {
	// BTC mainnet
	it('returns correct BTC address URL for mainnet', () => {
		const url = getExplorerUrl('BTC', 'address', 'bc1qtest', 'mainnet');
		expect(url).toBe('https://mempool.space/address/bc1qtest');
	});

	it('returns correct BTC tx URL for mainnet', () => {
		const url = getExplorerUrl('BTC', 'tx', 'abc123', 'mainnet');
		expect(url).toBe('https://mempool.space/tx/abc123');
	});

	// BTC testnet
	it('returns correct BTC address URL for testnet', () => {
		const url = getExplorerUrl('BTC', 'address', 'tb1qtest', 'testnet');
		expect(url).toBe('https://mempool.space/testnet/address/tb1qtest');
	});

	it('returns correct BTC tx URL for testnet', () => {
		const url = getExplorerUrl('BTC', 'tx', 'abc123', 'testnet');
		expect(url).toBe('https://mempool.space/testnet/tx/abc123');
	});

	// BSC mainnet
	it('returns correct BSC address URL for mainnet', () => {
		const url = getExplorerUrl('BSC', 'address', '0xabc', 'mainnet');
		expect(url).toBe('https://bscscan.com/address/0xabc');
	});

	it('returns correct BSC tx URL for mainnet', () => {
		const url = getExplorerUrl('BSC', 'tx', '0xtx', 'mainnet');
		expect(url).toBe('https://bscscan.com/tx/0xtx');
	});

	// BSC testnet
	it('returns correct BSC address URL for testnet', () => {
		const url = getExplorerUrl('BSC', 'address', '0xabc', 'testnet');
		expect(url).toBe('https://testnet.bscscan.com/address/0xabc');
	});

	it('returns correct BSC tx URL for testnet', () => {
		const url = getExplorerUrl('BSC', 'tx', '0xtx', 'testnet');
		expect(url).toBe('https://testnet.bscscan.com/tx/0xtx');
	});

	// SOL mainnet
	it('returns correct SOL account URL for mainnet', () => {
		const url = getExplorerUrl('SOL', 'address', 'SoLaddr', 'mainnet');
		expect(url).toBe('https://solscan.io/account/SoLaddr');
	});

	it('returns correct SOL tx URL for mainnet', () => {
		const url = getExplorerUrl('SOL', 'tx', 'SoLtx', 'mainnet');
		expect(url).toBe('https://solscan.io/tx/SoLtx');
	});

	// SOL testnet - adds ?cluster=devnet
	it('returns correct SOL account URL for testnet with devnet param', () => {
		const url = getExplorerUrl('SOL', 'address', 'SoLaddr', 'testnet');
		expect(url).toBe('https://solscan.io/account/SoLaddr?cluster=devnet');
	});

	it('returns correct SOL tx URL for testnet with devnet param', () => {
		const url = getExplorerUrl('SOL', 'tx', 'SoLtx', 'testnet');
		expect(url).toBe('https://solscan.io/tx/SoLtx?cluster=devnet');
	});

	// Default network
	it('defaults to mainnet when network not provided', () => {
		const url = getExplorerUrl('BTC', 'tx', 'abc123');
		expect(url).toBe('https://mempool.space/tx/abc123');
	});
});

describe('getExplorerTxUrl', () => {
	it('is a convenience wrapper for tx URLs', () => {
		expect(getExplorerTxUrl('BTC', 'abc123')).toBe('https://mempool.space/tx/abc123');
	});

	it('passes network through', () => {
		expect(getExplorerTxUrl('BSC', '0xtx', 'testnet')).toBe(
			'https://testnet.bscscan.com/tx/0xtx'
		);
	});
});
