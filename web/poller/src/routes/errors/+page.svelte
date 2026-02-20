<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { getDashboardErrors } from '$lib/utils/api';
	import { getTxExplorerUrl } from '$lib/utils/explorer';
	import { truncateAddress, formatTimestamp, chainBadgeClass } from '$lib/utils/formatting';
	import type { DashboardErrors } from '$lib/types';

	let data: DashboardErrors | null = $state(null);
	let loading = $state(true);
	let network = $state('mainnet');

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const res = await getDashboardErrors();
			data = res.data;
		} catch (err) {
			console.error('Failed to fetch errors', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		fetchData();
	});
</script>

<svelte:head>
	<title>Poller — Errors</title>
</svelte:head>

<Header title="Errors" subtitle="System health, discrepancies, and error log" />

<div class="page-body">
	{#if loading && !data}
		<div class="loading-state">Loading errors...</div>
	{:else if data}
		<!-- Section 1: Discrepancies -->
		<div class="card">
			<div class="card-header">
				<h2 class="card-title">Discrepancies</h2>
				<p class="card-subtitle">Auto-detected integrity issues</p>
			</div>
			{#if data.discrepancies.length === 0}
				<div class="card-empty">No discrepancies detected.</div>
			{:else}
				<div class="table-scroll">
					<table class="table">
						<thead>
							<tr>
								<th style="width: 20%;">Type</th>
								<th style="width: 22%;">Address</th>
								<th style="width: 10%;">Chain</th>
								<th style="width: 48%;">Details</th>
							</tr>
						</thead>
						<tbody>
							{#each data.discrepancies as row}
								<tr>
									<td>
										<span class="badge badge-error">{row.type}</span>
									</td>
									<td>
										{#if row.address}
											<span class="address-mono" title={row.address}>
												{truncateAddress(row.address, 6)}
											</span>
										{:else}
											<span class="text-muted">{'\u2014'}</span>
										{/if}
									</td>
									<td>
										{#if row.chain}
											<span class={chainBadgeClass(row.chain)}>{row.chain}</span>
										{:else}
											<span class="text-muted">{'\u2014'}</span>
										{/if}
									</td>
									<td>
										<span class="text-sm">
											{row.message}
											{#if row.calculated !== undefined && row.stored !== undefined}
												(Calculated: {row.calculated.toLocaleString()}, Stored: {row.stored.toLocaleString()})
											{/if}
										</span>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>

		<!-- Section 2: Stale Pending Transactions -->
		<div class="card">
			<div class="card-header">
				<h2 class="card-title">Stale Pending Transactions</h2>
				<p class="card-subtitle">Pending for more than 24 hours</p>
			</div>
			{#if data.stale_pending.length === 0}
				<div class="card-empty">No stale pending transactions.</div>
			{:else}
				<div class="table-scroll">
					<table class="table">
						<thead>
							<tr>
								<th style="width: 25%;">TX Hash</th>
								<th style="width: 10%;">Chain</th>
								<th style="width: 25%;">Address</th>
								<th style="width: 22%;">Detected At</th>
								<th style="width: 18%;">Hours Pending</th>
							</tr>
						</thead>
						<tbody>
							{#each data.stale_pending as row}
								<tr>
									<td>
										<a
											href={getTxExplorerUrl(row.chain, row.tx_hash, network)}
											target="_blank"
											rel="noopener noreferrer"
											class="hash-link"
											title={row.tx_hash}
										>
											{truncateAddress(row.tx_hash, 6)}
										</a>
									</td>
									<td>
										<span class={chainBadgeClass(row.chain)}>{row.chain}</span>
									</td>
									<td>
										<span class="address-mono" title={row.address}>
											{truncateAddress(row.address, 6)}
										</span>
									</td>
									<td>
										<span class="text-muted">{formatTimestamp(row.detected_at)}</span>
									</td>
									<td>
										<span class="text-error">{row.hours_pending}h</span>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>

		<!-- Section 3: System Errors -->
		<div class="card">
			<div class="card-header">
				<h2 class="card-title">System Errors</h2>
				<p class="card-subtitle">Provider failures and runtime issues</p>
			</div>
			{#if data.errors.length === 0}
				<div class="card-empty">No system errors recorded.</div>
			{:else}
				<div class="table-scroll">
					<table class="table">
						<thead>
							<tr>
								<th style="width: 10%;">Severity</th>
								<th style="width: 12%;">Category</th>
								<th style="width: 40%;">Message</th>
								<th style="width: 22%;">Timestamp</th>
								<th style="width: 16%;">Resolved</th>
							</tr>
						</thead>
						<tbody>
							{#each data.errors as err}
								<tr>
									<td>
										<div class="severity-indicator">
											<span
												class="severity-dot"
												class:severity-error={err.severity === 'ERROR'}
												class:severity-warning={err.severity === 'WARNING'}
												class:severity-info={err.severity === 'INFO'}
											></span>
											<span class="text-sm">{err.severity}</span>
										</div>
									</td>
									<td>
										<span class="badge badge-default">{err.category}</span>
									</td>
									<td>
										<span class="text-sm">{err.message}</span>
									</td>
									<td>
										<span class="text-muted">{formatTimestamp(err.created_at)}</span>
									</td>
									<td>
										{#if err.resolved}
											<span class="text-success">Yes</span>
										{:else}
											<span class="text-error">No</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.page-body {
		padding: 1.5rem 2rem;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.loading-state {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		padding: 2rem 0;
	}

	/* Card */
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.card-header {
		padding: 1rem 1.25rem;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.card-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.card-subtitle {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin: 0.25rem 0 0 0;
	}

	.card-empty {
		padding: 1.5rem 1.25rem;
		color: var(--color-text-muted);
		font-size: 0.8125rem;
	}

	/* Table */
	.table-scroll {
		overflow-x: auto;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8125rem;
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
		background: var(--color-bg-elevated);
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

	/* Helpers */
	.text-sm {
		font-size: 0.8125rem;
	}

	.text-muted {
		color: var(--color-text-muted);
	}

	.text-error {
		color: var(--color-error);
		font-weight: 500;
	}

	.text-success {
		color: var(--color-success);
		font-weight: 500;
	}

	.address-mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		color: var(--color-text-primary);
	}

	.hash-link {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		color: var(--color-accent-text);
		text-decoration: none;
	}

	.hash-link:hover {
		color: var(--color-accent-hover);
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

	.badge-error {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.badge-warning {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.badge-default {
		background: var(--color-bg-surface-active);
		color: var(--color-text-secondary);
	}

	/* Severity */
	.severity-indicator {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.severity-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.severity-error {
		background: var(--color-error);
	}

	.severity-warning {
		background: var(--color-warning);
	}

	.severity-info {
		background: var(--color-info);
	}
</style>
