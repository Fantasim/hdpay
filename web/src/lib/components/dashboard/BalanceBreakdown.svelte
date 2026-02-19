<script lang="ts">
	import type { ChainPortfolio, Chain } from '$lib/types';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import { formatRawBalance, formatUsd, formatNumber } from '$lib/utils/formatting';

	interface Props {
		chains: ChainPortfolio[];
	}

	let { chains }: Props = $props();

	// Flatten chain tokens into table rows.
	interface BalanceRow {
		chain: Chain;
		displayToken: string;
		rawToken: string;
		balance: string;
		usd: number;
		fundedCount: number;
	}

	let rows = $derived<BalanceRow[]>(() => {
		const result: BalanceRow[] = [];
		for (const chainData of chains) {
			for (const token of chainData.tokens) {
				// Map NATIVE token to the chain's native symbol for display.
				const displayToken = token.symbol === 'NATIVE'
					? CHAIN_NATIVE_SYMBOLS[chainData.chain]
					: token.symbol;

				result.push({
					chain: chainData.chain,
					displayToken,
					rawToken: token.symbol,
					balance: token.balance,
					usd: token.usd,
					fundedCount: token.fundedCount
				});
			}
		}
		return result;
	});

	function badgeClass(chain: Chain): string {
		return `badge badge-${chain.toLowerCase()}`;
	}
</script>

<div class="section">
	<div class="section-title">Balance Breakdown</div>

	{#if rows().length === 0}
		<div class="empty-state">
			<p>No balances found. Run a scan to discover funded addresses.</p>
		</div>
	{:else}
		<div class="table-wrapper">
			<table class="table">
				<thead>
					<tr>
						<th>Chain</th>
						<th>Token</th>
						<th class="text-right">Balance</th>
						<th class="text-right">USD Value</th>
						<th class="text-right">Funded Addresses</th>
					</tr>
				</thead>
				<tbody>
					{#each rows() as row (row.chain + row.rawToken)}
						<tr>
							<td><span class={badgeClass(row.chain)}>{row.chain}</span></td>
							<td>{row.displayToken}</td>
							<td class="mono text-right">{formatRawBalance(row.balance, row.chain, row.rawToken)}</td>
							<td class="mono text-right">{formatUsd(row.usd)}</td>
							<td class="text-right">{formatNumber(row.fundedCount)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.section {
		margin-bottom: 2rem;
	}

	.section-title {
		font-size: 1.125rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
		margin-bottom: 1rem;
	}

	.table-wrapper {
		border: 1px solid var(--color-border);
		border-radius: 8px;
		overflow: hidden;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	.table thead {
		background: var(--color-bg-secondary);
	}

	.table th {
		padding: 0.625rem 1rem;
		font-weight: 500;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		text-align: left;
		border-bottom: 1px solid var(--color-border);
	}

	.table td {
		padding: 0.625rem 1rem;
		color: var(--color-text-secondary);
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.table tr:last-child td {
		border-bottom: none;
	}

	.table tr:hover td {
		background: var(--color-bg-surface-hover);
	}

	.text-right {
		text-align: right;
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		font-size: 0.6875rem;
		font-weight: 600;
		letter-spacing: 0.05em;
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

	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 3rem;
		border: 1px dashed var(--color-border);
		border-radius: 8px;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
