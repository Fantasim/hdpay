<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { getPoints, getPendingPoints } from '$lib/utils/api';
	import { truncateAddress, formatPoints, formatTimestamp, chainBadgeClass } from '$lib/utils/formatting';
	import type { PointsWithTransactions } from '$lib/types';

	interface PointsRow {
		address: string;
		chain: string;
		unclaimed: number;
		pending: number;
		txCount: number;
		lastTx: string | null;
		total: number;
	}

	let rows: PointsRow[] = $state([]);
	let loading = $state(true);

	// Summary values
	let totalUnclaimed = $derived(rows.reduce((sum, r) => sum + r.unclaimed, 0));
	let totalPending = $derived(rows.reduce((sum, r) => sum + r.pending, 0));
	let totalAllTime = $derived(rows.reduce((sum, r) => sum + r.total, 0));
	let unclaimedAccounts = $derived(rows.filter((r) => r.unclaimed > 0).length);
	let pendingTxCount = $derived(rows.filter((r) => r.pending > 0).length);

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const [pointsRes, pendingRes] = await Promise.all([getPoints(), getPendingPoints()]);
			const pointsData = pointsRes.data;
			const pendingData = pendingRes.data;

			// Build a map of pending points by address+chain
			const pendingMap = new Map<string, number>();
			for (const p of pendingData) {
				pendingMap.set(`${p.address}:${p.chain}`, p.pending_points);
			}

			// Build rows from points data
			rows = pointsData.map((p: PointsWithTransactions) => {
				const key = `${p.address}:${p.chain}`;
				const txs = p.transactions ?? [];
				const lastTxDate =
					txs.length > 0 ? txs[txs.length - 1].detected_at : null;
				return {
					address: p.address,
					chain: p.chain,
					unclaimed: p.unclaimed,
					pending: pendingMap.get(key) ?? 0,
					txCount: txs.length,
					lastTx: lastTxDate,
					total: p.total
				};
			});
		} catch (err) {
			console.error('Failed to fetch points data', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		fetchData();
	});
</script>

<svelte:head>
	<title>Poller — Points</title>
</svelte:head>

<Header title="Points" subtitle="Accounts with unclaimed and pending points" />

<div class="page-body">
	{#if loading && rows.length === 0}
		<div class="loading-state">Loading points...</div>
	{:else}
		<!-- Summary Cards -->
		<div class="summary-cards">
			<div class="summary-card">
				<div class="summary-card-label">Total Unclaimed</div>
				<div class="summary-card-value">{formatPoints(totalUnclaimed)}</div>
				<div class="summary-card-meta">across {unclaimedAccounts} account{unclaimedAccounts !== 1 ? 's' : ''}</div>
			</div>
			<div class="summary-card">
				<div class="summary-card-label">Total Pending</div>
				<div class="summary-card-value">{formatPoints(totalPending)}</div>
				<div class="summary-card-meta">{pendingTxCount} unconfirmed tx{pendingTxCount !== 1 ? 's' : ''}</div>
			</div>
			<div class="summary-card">
				<div class="summary-card-label">All-Time Awarded</div>
				<div class="summary-card-value">{formatPoints(totalAllTime)}</div>
				<div class="summary-card-meta">since inception</div>
			</div>
		</div>

		<!-- Points Table -->
		{#if rows.length === 0}
			<div class="empty-state">No points accounts found.</div>
		{:else}
			<div class="table-wrapper">
				<table class="table">
					<thead>
						<tr>
							<th>Address</th>
							<th>Chain</th>
							<th class="text-right">Unclaimed</th>
							<th class="text-right">Pending</th>
							<th class="text-center">TX Count</th>
							<th>Last TX</th>
							<th class="text-right">All-Time Total</th>
						</tr>
					</thead>
					<tbody>
						{#each rows as row}
							<tr>
								<td>
									<span class="address-mono" title={row.address}>
										{truncateAddress(row.address, 6)}
									</span>
								</td>
								<td>
									<span class={chainBadgeClass(row.chain)}>{row.chain}</span>
								</td>
								<td class="text-right">
									<span
										class="points-value"
										class:points-success={row.unclaimed > 0}
										class:points-muted={row.unclaimed === 0}
									>
										{formatPoints(row.unclaimed)}
									</span>
								</td>
								<td class="text-right">
									<span
										class="points-value"
										class:points-warning={row.pending > 0}
										class:points-muted={row.pending === 0}
									>
										{formatPoints(row.pending)}
									</span>
								</td>
								<td class="text-center">{row.txCount}</td>
								<td class="text-sm">{formatTimestamp(row.lastTx)}</td>
								<td class="text-right">
									<span class="points-value">{formatPoints(row.total)}</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</div>

<style>
	.page-body {
		padding: 1.5rem 2rem;
	}

	.loading-state,
	.empty-state {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		padding: 2rem 0;
	}

	/* Summary Cards */
	.summary-cards {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 1rem;
		margin-bottom: 1.5rem;
	}

	.summary-card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		padding: 1.25rem;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.summary-card-label {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.summary-card-value {
		font-size: 1.5rem;
		font-weight: 600;
		font-family: var(--font-mono);
		color: var(--color-text-primary);
		letter-spacing: -0.02em;
	}

	.summary-card-meta {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		margin-top: 0.25rem;
	}

	/* Table */
	.table-wrapper {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8125rem;
	}

	.table thead {
		background: var(--color-bg-elevated);
	}

	.table th {
		padding: 0.625rem 0.75rem;
		text-align: left;
		font-weight: 500;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		white-space: nowrap;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.table td {
		padding: 0.625rem 0.75rem;
		border-bottom: 1px solid var(--color-border-subtle);
		vertical-align: middle;
	}

	.table tbody tr:hover {
		background: var(--color-bg-surface-hover);
	}

	.table tbody tr:last-child td {
		border-bottom: none;
	}

	/* Address */
	.address-mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		color: var(--color-text-primary);
	}

	/* Points */
	.points-value {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		font-weight: 500;
	}

	.points-success {
		color: var(--color-success);
	}

	.points-warning {
		color: var(--color-warning);
	}

	.points-muted {
		color: var(--color-text-muted);
	}

	/* Alignment */
	.text-right {
		text-align: right;
	}

	.text-center {
		text-align: center;
	}

	.text-sm {
		font-size: 0.8125rem;
	}

	/* Badges */
	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: var(--radius-md);
		font-size: 0.6875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		white-space: nowrap;
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
</style>
