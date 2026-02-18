import { ADDRESS_TRUNCATE_LENGTH, BALANCE_DECIMAL_PLACES } from '$lib/constants';

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
 */
export function formatDate(dateStr: string): string {
	const date = new Date(dateStr);
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
