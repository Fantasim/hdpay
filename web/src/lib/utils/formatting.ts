import { ADDRESS_TRUNCATE_LENGTH, BALANCE_DECIMAL_PLACES, TOKEN_DECIMALS } from '$lib/constants';
import type { Chain } from '$lib/types';

/**
 * Truncate an address to show start...end
 */
export function truncateAddress(address: string, length: number = ADDRESS_TRUNCATE_LENGTH): string {
	if (address.length <= length * 2 + 3) return address;
	return `${address.slice(0, length)}...${address.slice(-length)}`;
}

/**
 * Format a balance string to a fixed number of decimal places.
 */
export function formatBalance(balance: string, decimals: number = BALANCE_DECIMAL_PLACES): string {
	const num = parseFloat(balance);
	if (isNaN(num)) return '0';
	if (num === 0) return '0';
	return num.toFixed(decimals).replace(/\.?0+$/, '');
}

/**
 * Convert a raw balance (satoshis/wei/lamports) to human-readable units
 * based on the chain and token, then format for display.
 * Uses string-based decimal placement to avoid float precision loss
 * for large integer strings (>2^53).
 */
export function formatRawBalance(rawBalance: string, chain: Chain, token: string): string {
	const decimals = TOKEN_DECIMALS[chain]?.[token] ?? 0;

	// Strip leading zeros but keep at least one digit.
	let raw = rawBalance.replace(/^0+/, '') || '0';
	if (raw === '0') return '0';

	if (decimals === 0) return raw;

	// Pad with leading zeros if the raw string is shorter than decimals.
	while (raw.length <= decimals) {
		raw = '0' + raw;
	}

	const intPart = raw.slice(0, raw.length - decimals);
	const fracPart = raw.slice(raw.length - decimals);

	// Trim trailing zeros from the fractional part, keep up to 8 digits.
	const trimmedFrac = fracPart.slice(0, Math.min(decimals, 8)).replace(/0+$/, '');
	if (!trimmedFrac) return intPart;
	return `${intPart}.${trimmedFrac}`;
}

/**
 * Format a USD amount with $ prefix and 2 decimal places.
 */
export function formatUsd(amount: number): string {
	return `$${amount.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

/**
 * Format a large number with commas.
 */
export function formatNumber(n: number): string {
	return n.toLocaleString('en-US');
}

/**
 * Format a date string to locale display.
 * Returns 'N/A' for invalid or missing date strings.
 */
export function formatDate(dateStr: string): string {
	if (!dateStr) return 'N/A';
	const date = new Date(dateStr);
	if (isNaN(date.getTime())) return 'N/A';
	return date.toLocaleDateString('en-US', {
		month: 'short',
		day: 'numeric',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

/**
 * Format elapsed time string (e.g., "2m30s") to a more readable form.
 */
export function formatElapsed(elapsed: string): string {
	return elapsed;
}

/**
 * Format a date string as relative time ("2 min ago", "1 hour ago", "Never").
 */
export function formatRelativeTime(dateStr: string | null): string {
	if (!dateStr) return 'Never';

	const date = new Date(dateStr);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);

	if (diffSec < 60) return 'just now';

	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin} min ago`;

	const diffHours = Math.floor(diffMin / 60);
	if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;

	const diffDays = Math.floor(diffHours / 24);
	if (diffDays < 30) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;

	return formatDate(dateStr);
}

/**
 * Copy text to the clipboard. Returns true on success.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
	try {
		await navigator.clipboard.writeText(text);
		return true;
	} catch {
		return false;
	}
}
