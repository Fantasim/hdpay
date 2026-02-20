import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	truncateAddress,
	formatBalance,
	formatRawBalance,
	formatUsd,
	formatNumber,
	formatDate,
	formatRelativeTime,
	parseElapsedToMs,
	formatDuration,
	isZeroBalance,
	computeUsdValue
} from './formatting';

describe('truncateAddress', () => {
	it('does not truncate short addresses', () => {
		expect(truncateAddress('0x1234567890')).toBe('0x1234567890');
	});

	it('truncates long addresses with default length', () => {
		const addr = '0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb';
		const result = truncateAddress(addr);
		expect(result).toBe('0xF278cF...f25C1cdb');
		expect(result).toContain('...');
	});

	it('uses custom truncation length', () => {
		const addr = 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu';
		const result = truncateAddress(addr, 6);
		expect(result).toBe('bc1qcr...306fyu');
	});

	it('returns address as-is when exactly at boundary', () => {
		// length*2+3 = 8*2+3 = 19 chars; an address of 19 chars or less is not truncated
		const addr = '0x1234567890abcdefg'; // 19 chars
		expect(truncateAddress(addr)).toBe(addr);
	});
});

describe('formatBalance', () => {
	it('returns 0 for NaN', () => {
		expect(formatBalance('not-a-number')).toBe('0');
	});

	it('returns 0 for zero', () => {
		expect(formatBalance('0')).toBe('0');
	});

	it('formats with default decimal places and strips trailing zeros', () => {
		expect(formatBalance('1.500000')).toBe('1.5');
	});

	it('formats small numbers', () => {
		expect(formatBalance('0.000001')).toBe('0.000001');
	});

	it('formats large numbers', () => {
		expect(formatBalance('123456.789')).toBe('123456.789');
	});

	it('uses custom decimal places', () => {
		expect(formatBalance('1.23456789', 2)).toBe('1.23');
	});

	it('strips trailing decimal point', () => {
		expect(formatBalance('1.000000')).toBe('1');
	});
});

describe('formatRawBalance', () => {
	it('returns 0 for empty string', () => {
		expect(formatRawBalance('', 'BTC', 'NATIVE')).toBe('0');
	});

	it('returns 0 for "0"', () => {
		expect(formatRawBalance('0', 'BTC', 'NATIVE')).toBe('0');
	});

	// BTC: 8 decimals
	it('formats BTC satoshis correctly', () => {
		// 100000000 satoshis = 1 BTC
		expect(formatRawBalance('100000000', 'BTC', 'NATIVE')).toBe('1');
	});

	it('formats small BTC amount', () => {
		// 168841 satoshis = 0.00168841 BTC
		expect(formatRawBalance('168841', 'BTC', 'NATIVE')).toBe('0.00168841');
	});

	// BSC: 18 decimals for native
	it('formats BSC native (18 decimals)', () => {
		// 1 BNB = 1000000000000000000 wei
		expect(formatRawBalance('1000000000000000000', 'BSC', 'NATIVE')).toBe('1');
	});

	it('formats BSC USDC (18 decimals)', () => {
		// 5000000000000000000 = 5 USDC on BSC
		expect(formatRawBalance('5000000000000000000', 'BSC', 'USDC')).toBe('5');
	});

	it('formats BSC USDT (18 decimals)', () => {
		expect(formatRawBalance('1500000000000000000', 'BSC', 'USDT')).toBe('1.5');
	});

	// SOL: 9 decimals for native
	it('formats SOL native (9 decimals)', () => {
		// 1 SOL = 1000000000 lamports
		expect(formatRawBalance('1000000000', 'SOL', 'NATIVE')).toBe('1');
	});

	it('formats SOL with fractional lamports', () => {
		expect(formatRawBalance('9000000000', 'SOL', 'NATIVE')).toBe('9');
	});

	// SOL USDC: 6 decimals
	it('formats SOL USDC (6 decimals)', () => {
		// 20000000 = 20 USDC
		expect(formatRawBalance('20000000', 'SOL', 'USDC')).toBe('20');
	});

	it('formats SOL USDT (6 decimals)', () => {
		expect(formatRawBalance('1500000', 'SOL', 'USDT')).toBe('1.5');
	});

	// SQLite ".0" suffix edge case
	it('handles SQLite CAST ".0" suffix for BTC', () => {
		expect(formatRawBalance('168841.0', 'BTC', 'NATIVE')).toBe('0.00168841');
	});

	it('handles SQLite CAST ".0" suffix for large BSC wei', () => {
		expect(formatRawBalance('1000000000000000000.0', 'BSC', 'NATIVE')).toBe('1');
	});

	// Large integers beyond float precision
	it('handles large integer strings without precision loss', () => {
		// 999999999999999999999 wei = 999.999999999999999999 BNB
		// String-based math should not lose precision
		const result = formatRawBalance('999999999999999999999', 'BSC', 'NATIVE');
		expect(result).toBe('999.99999999');
	});

	// Edge: raw balance shorter than decimals
	it('handles raw balance shorter than decimal count', () => {
		// 1 satoshi = 0.00000001 BTC
		expect(formatRawBalance('1', 'BTC', 'NATIVE')).toBe('0.00000001');
	});

	// Unknown token defaults to 0 decimals
	it('handles unknown token (0 decimals)', () => {
		expect(formatRawBalance('42', 'BTC', 'UNKNOWN')).toBe('42');
	});

	it('strips leading zeros from raw input', () => {
		expect(formatRawBalance('000168841', 'BTC', 'NATIVE')).toBe('0.00168841');
	});
});

describe('formatUsd', () => {
	it('formats zero', () => {
		expect(formatUsd(0)).toBe('$0.00');
	});

	it('formats positive amount', () => {
		expect(formatUsd(42.5)).toBe('$42.50');
	});

	it('formats large numbers with commas', () => {
		const result = formatUsd(1234567.89);
		expect(result).toBe('$1,234,567.89');
	});

	it('rounds to 2 decimal places', () => {
		expect(formatUsd(1.999)).toBe('$2.00');
	});
});

describe('formatNumber', () => {
	it('formats with commas', () => {
		expect(formatNumber(1234567)).toBe('1,234,567');
	});

	it('does not add commas for small numbers', () => {
		expect(formatNumber(999)).toBe('999');
	});
});

describe('formatDate', () => {
	it('returns N/A for empty string', () => {
		expect(formatDate('')).toBe('N/A');
	});

	it('returns N/A for invalid date', () => {
		expect(formatDate('not-a-date')).toBe('N/A');
	});

	it('formats valid ISO date string', () => {
		const result = formatDate('2026-02-18T10:30:00Z');
		// Just verify it contains expected parts (locale-dependent formatting)
		expect(result).not.toBe('N/A');
		expect(result).toContain('2026');
		expect(result).toContain('Feb');
	});
});

describe('formatRelativeTime', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-02-19T12:00:00Z'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('returns Never for null', () => {
		expect(formatRelativeTime(null)).toBe('Never');
	});

	it('returns "just now" for recent timestamps', () => {
		expect(formatRelativeTime('2026-02-19T11:59:30Z')).toBe('just now');
	});

	it('returns minutes ago', () => {
		expect(formatRelativeTime('2026-02-19T11:55:00Z')).toBe('5 min ago');
	});

	it('returns hours ago (singular)', () => {
		expect(formatRelativeTime('2026-02-19T11:00:00Z')).toBe('1 hour ago');
	});

	it('returns hours ago (plural)', () => {
		expect(formatRelativeTime('2026-02-19T09:00:00Z')).toBe('3 hours ago');
	});

	it('returns days ago (singular)', () => {
		expect(formatRelativeTime('2026-02-18T12:00:00Z')).toBe('1 day ago');
	});

	it('returns days ago (plural)', () => {
		expect(formatRelativeTime('2026-02-14T12:00:00Z')).toBe('5 days ago');
	});

	it('returns formatted date for old timestamps (>30 days)', () => {
		const result = formatRelativeTime('2026-01-01T00:00:00Z');
		expect(result).not.toBe('Never');
		expect(result).toContain('2026');
	});
});

describe('parseElapsedToMs', () => {
	it('parses seconds only', () => {
		expect(parseElapsedToMs('45s')).toBe(45000);
	});

	it('parses minutes and seconds', () => {
		expect(parseElapsedToMs('2m30s')).toBe(150000);
	});

	it('parses hours, minutes, and seconds', () => {
		expect(parseElapsedToMs('1h5m10s')).toBe(3910000);
	});

	it('parses hours only', () => {
		expect(parseElapsedToMs('2h')).toBe(7200000);
	});

	it('parses minutes only', () => {
		expect(parseElapsedToMs('10m')).toBe(600000);
	});

	it('returns 0 for empty string', () => {
		expect(parseElapsedToMs('')).toBe(0);
	});

	it('returns 0 for non-matching string', () => {
		expect(parseElapsedToMs('unknown')).toBe(0);
	});
});

describe('formatDuration', () => {
	it('formats seconds', () => {
		expect(formatDuration(5000)).toBe('5s');
	});

	it('formats minutes and seconds', () => {
		expect(formatDuration(150000)).toBe('2m 30s');
	});

	it('formats exact minutes', () => {
		expect(formatDuration(120000)).toBe('2m');
	});

	it('formats hours and minutes', () => {
		expect(formatDuration(3900000)).toBe('1h 5m');
	});

	it('rounds up partial seconds', () => {
		expect(formatDuration(1500)).toBe('2s');
	});

	it('formats sub-second as 1s', () => {
		expect(formatDuration(100)).toBe('1s');
	});
});

describe('isZeroBalance', () => {
	it('returns true for "0"', () => {
		expect(isZeroBalance('0')).toBe(true);
	});

	it('returns true for "0.0"', () => {
		expect(isZeroBalance('0.0')).toBe(true);
	});

	it('returns true for empty string (NaN)', () => {
		expect(isZeroBalance('')).toBe(true);
	});

	it('returns true for non-numeric string', () => {
		expect(isZeroBalance('abc')).toBe(true);
	});

	it('returns false for positive balance', () => {
		expect(isZeroBalance('100000')).toBe(false);
	});

	it('returns false for small positive balance', () => {
		expect(isZeroBalance('0.001')).toBe(false);
	});
});

describe('computeUsdValue', () => {
	const prices = { BTC: 50000, BNB: 300, SOL: 100, USDC: 1, USDT: 1 };

	it('returns null when decimals are 0 (unknown token)', () => {
		expect(computeUsdValue('100', 'BTC', 'UNKNOWN', prices)).toBeNull();
	});

	it('computes BTC native value', () => {
		// 1 BTC = 100000000 satoshis, price $50000 → $50,000.00
		const result = computeUsdValue('100000000', 'BTC', 'NATIVE', prices);
		expect(result).toBe('$50,000.00');
	});

	it('computes BSC native value (uses BNB key)', () => {
		// 1 BNB = 1e18 wei, price $300 → $300.00
		const result = computeUsdValue('1000000000000000000', 'BSC', 'NATIVE', prices);
		expect(result).toBe('$300.00');
	});

	it('computes SOL USDC value', () => {
		// 20 USDC = 20000000 (6 decimals), price $1 → $20.00
		const result = computeUsdValue('20000000', 'SOL', 'USDC', prices);
		expect(result).toBe('$20.00');
	});

	it('returns null for zero amount', () => {
		expect(computeUsdValue('0', 'BTC', 'NATIVE', prices)).toBeNull();
	});

	it('returns null when price is missing', () => {
		const partialPrices = { BTC: 50000 };
		expect(computeUsdValue('1000000000', 'SOL', 'NATIVE', partialPrices)).toBeNull();
	});

	it('returns null for NaN amount', () => {
		expect(computeUsdValue('abc', 'BTC', 'NATIVE', prices)).toBeNull();
	});
});
