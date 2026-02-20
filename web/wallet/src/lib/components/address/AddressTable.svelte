<script lang="ts">
	import type { AddressWithBalance, Chain } from '$lib/types';
	import { truncateAddress, formatRawBalance, formatRelativeTime, copyToClipboard, isZeroBalance } from '$lib/utils/formatting';
	import { CHAIN_COLORS } from '$lib/constants';

	interface Props {
		addresses: AddressWithBalance[];
		loading: boolean;
	}

	let { addresses, loading }: Props = $props();

	let copiedIndex: number | null = $state(null);
	let copyTimer: ReturnType<typeof setTimeout> | null = null;

	async function handleCopy(address: string, index: number): Promise<void> {
		const ok = await copyToClipboard(address);
		if (ok) {
			if (copyTimer !== null) clearTimeout(copyTimer);
			copiedIndex = index;
			copyTimer = setTimeout(() => {
				copiedIndex = null;
				copyTimer = null;
			}, 1500);
		}
	}

	function chainBadgeClass(chain: string): string {
		return `badge badge-${chain.toLowerCase()}`;
	}
</script>

<div class="table-wrapper">
	{#if loading}
		<div class="table-loading">
			<span class="loading-text">Loading addresses...</span>
		</div>
	{:else if addresses.length === 0}
		<div class="table-empty">
			<span class="empty-text">No addresses found</span>
		</div>
	{:else}
		<table class="table">
			<thead>
				<tr>
					<th>#</th>
					<th>Chain</th>
					<th>Address</th>
					<th class="text-right">Native Balance</th>
					<th>Token Balances</th>
					<th>Last Scanned</th>
				</tr>
			</thead>
			<tbody>
				{#each addresses as addr (addr.chain + '-' + addr.addressIndex)}
					<tr>
						<td class="text-muted">{addr.addressIndex}</td>
						<td>
							<span class={chainBadgeClass(addr.chain)}>{addr.chain}</span>
						</td>
						<td>
							<div class="address-cell">
								<span class="mono">{truncateAddress(addr.address)}</span>
								<button
									class="copy-btn"
									title={copiedIndex === addr.addressIndex ? 'Copied!' : 'Copy address'}
									onclick={() => handleCopy(addr.address, addr.addressIndex)}
								>
									{#if copiedIndex === addr.addressIndex}
										<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--color-success)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
											<path d="M4 8l3 3 5-6"/>
										</svg>
									{:else}
										<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
											<rect x="5" y="5" width="9" height="9" rx="1"/>
											<path d="M11 5V3a1 1 0 0 0-1-1H3a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h2"/>
										</svg>
									{/if}
								</button>
							</div>
						</td>
						<td class="text-right">
							<span class="mono" class:balance-zero={isZeroBalance(addr.nativeBalance)}>
								{formatRawBalance(addr.nativeBalance, addr.chain as Chain, 'NATIVE')}
							</span>
						</td>
						<td>
							{#if addr.tokenBalances.length > 0}
								<div class="token-balances">
									{#each addr.tokenBalances as tb}
										<div class="token-balance-row">
											<span class="token-label">{tb.symbol}</span>
											<span class="token-amount mono">{formatRawBalance(tb.balance, addr.chain as Chain, tb.symbol)}</span>
										</div>
									{/each}
								</div>
							{:else}
								<span class="text-muted text-sm">&mdash;</span>
							{/if}
						</td>
						<td>
							{#if addr.lastScanned}
								<span class="time-muted">{formatRelativeTime(addr.lastScanned)}</span>
							{:else}
								<span class="time-never">Never</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<style>
	.table-wrapper {
		border: 1px solid var(--color-border);
		border-radius: 8px;
		overflow: hidden;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8125rem;
	}

	.table th {
		text-align: left;
		padding: 0.625rem 1rem;
		font-size: 0.6875rem;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--color-text-muted);
		background: var(--color-bg-secondary);
		border-bottom: 1px solid var(--color-border);
		position: sticky;
		top: 0;
		z-index: 1;
	}

	.table td {
		padding: 0.5rem 1rem;
		border-bottom: 1px solid var(--color-border-subtle);
		color: var(--color-text-secondary);
		vertical-align: middle;
	}

	.table tr:last-child td {
		border-bottom: none;
	}

	.table tr:hover td {
		background: var(--color-bg-surface-hover);
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.text-right {
		text-align: right;
	}

	.text-muted {
		color: var(--color-text-muted);
	}

	.text-sm {
		font-size: 0.75rem;
	}

	.balance-zero {
		color: var(--color-text-disabled);
	}

	.address-cell {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.copy-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 4px;
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		transition: all 150ms ease;
		padding: 0;
		flex-shrink: 0;
	}

	.copy-btn:hover {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-primary);
	}

	.token-balances {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
	}

	.token-balance-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.75rem;
	}

	.token-label {
		color: var(--color-text-muted);
		font-weight: 500;
		min-width: 36px;
	}

	.token-amount {
		color: var(--color-text-secondary);
		font-size: 0.75rem;
	}

	.time-muted {
		color: var(--color-text-muted);
		font-size: 0.8125rem;
	}

	.time-never {
		color: var(--color-text-disabled);
		font-style: italic;
		font-size: 0.8125rem;
	}

	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		font-size: 0.6875rem;
		font-weight: 600;
		letter-spacing: 0.02em;
	}

	.badge-btc {
		background: var(--color-btc-muted);
		color: var(--color-btc);
	}

	.badge-bsc {
		background: var(--color-bsc-muted);
		color: var(--color-bsc);
	}

	.badge-sol {
		background: var(--color-sol-muted);
		color: var(--color-sol);
	}

	.table-loading,
	.table-empty {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 3rem;
	}

	.loading-text,
	.empty-text {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
