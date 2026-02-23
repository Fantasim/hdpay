<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { getDashboardTransactions } from '$lib/utils/api';
	import { getTxExplorerUrl } from '$lib/utils/explorer';
	import {
		truncateAddress,
		formatUsd,
		formatPoints,
		formatTimestamp,
		chainBadgeClass
	} from '$lib/utils/formatting';
	import {
		SUPPORTED_CHAINS,
		ALL_TOKENS,
		TX_STATUSES,
		TABLE_PAGE_SIZES,
		TABLE_DEFAULT_PAGE_SIZE,
		MAX_TIER_INDEX
	} from '$lib/constants';
	import type { Transaction, TransactionFilters, PaginationMeta } from '$lib/types';

	// Filter state
	let filterChain = $state('');
	let filterToken = $state('');
	let filterStatus = $state('');
	let filterTier = $state('');
	let filterMinUsd = $state('');
	let filterMaxUsd = $state('');

	// Pagination state
	let pageSize = $state(TABLE_DEFAULT_PAGE_SIZE);
	let currentPage = $state(1);

	// Data state
	let transactions: Transaction[] = $state([]);
	let meta: PaginationMeta | null = $state(null);
	let loading = $state(true);
	let network = $state('mainnet');

	async function fetchNetwork(): Promise<void> {
		try {
			const { getHealth } = await import('$lib/utils/api');
			const res = await getHealth();
			network = res.data.network ?? 'mainnet';
		} catch {
			// fallback to mainnet
		}
	}

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const filters: TransactionFilters = {
				page: currentPage,
				page_size: pageSize
			};
			if (filterChain) filters.chain = filterChain;
			if (filterToken) filters.token = filterToken;
			if (filterStatus) filters.status = filterStatus as 'PENDING' | 'CONFIRMED';
			if (filterTier) filters.tier = Number(filterTier);
			if (filterMinUsd) filters.min_usd = Number(filterMinUsd);
			if (filterMaxUsd) filters.max_usd = Number(filterMaxUsd);

			const res = await getDashboardTransactions(filters);
			transactions = res.data ?? [];
			meta = res.meta ?? null;
		} catch (err) {
			console.error('Failed to fetch transactions', err);
		} finally {
			loading = false;
		}
	}

	function applyFilters(): void {
		currentPage = 1;
		fetchData();
	}

	function setPageSize(size: number): void {
		pageSize = size;
		currentPage = 1;
		fetchData();
	}

	function goToPage(page: number): void {
		currentPage = page;
		fetchData();
	}

	let totalPages = $derived(meta ? Math.ceil(meta.total / meta.page_size) : 0);
	let showFrom = $derived(meta ? (meta.page - 1) * meta.page_size + 1 : 0);
	let showTo = $derived(meta ? Math.min(meta.page * meta.page_size, meta.total) : 0);

	let pageNumbers = $derived.by(() => {
		if (!totalPages) return [];
		const pages: number[] = [];
		const maxVisible = 5;
		let start = Math.max(1, currentPage - Math.floor(maxVisible / 2));
		let end = Math.min(totalPages, start + maxVisible - 1);
		if (end - start + 1 < maxVisible) {
			start = Math.max(1, end - maxVisible + 1);
		}
		for (let i = start; i <= end; i++) {
			pages.push(i);
		}
		return pages;
	});

	let tierRange = $derived(
		Array.from({ length: MAX_TIER_INDEX + 1 }, (_, i) => i)
	);

	onMount(() => {
		fetchNetwork();
		fetchData();
	});
</script>

<svelte:head>
	<title>Poller — Transactions</title>
</svelte:head>

<Header title="Transactions" subtitle="Full transaction history" />

<div class="page-body">
	<!-- Filter Bar -->
	<div class="filter-bar">
		<select
			class="form-select"
			bind:value={filterChain}
			onchange={applyFilters}
			aria-label="Filter by chain"
		>
			<option value="">All Chains</option>
			{#each SUPPORTED_CHAINS as chain}
				<option value={chain}>{chain}</option>
			{/each}
		</select>

		<select
			class="form-select"
			bind:value={filterToken}
			onchange={applyFilters}
			aria-label="Filter by token"
		>
			<option value="">All Tokens</option>
			{#each ALL_TOKENS as token}
				<option value={token}>{token}</option>
			{/each}
		</select>

		<select
			class="form-select"
			bind:value={filterStatus}
			onchange={applyFilters}
			aria-label="Filter by status"
		>
			<option value="">All</option>
			{#each TX_STATUSES as status}
				<option value={status}>{status.charAt(0) + status.slice(1).toLowerCase()}</option>
			{/each}
		</select>

		<select
			class="form-select"
			bind:value={filterTier}
			onchange={applyFilters}
			aria-label="Filter by tier"
		>
			<option value="">All Tiers</option>
			{#each tierRange as t}
				<option value={String(t)}>Tier {t}</option>
			{/each}
		</select>

		<div class="toolbar-separator"></div>

		<input
			class="form-input form-input-narrow"
			type="number"
			placeholder="Min $"
			min="0"
			step="0.01"
			bind:value={filterMinUsd}
			onchange={applyFilters}
			aria-label="Minimum USD value"
		/>

		<input
			class="form-input form-input-narrow"
			type="number"
			placeholder="Max $"
			min="0"
			step="0.01"
			bind:value={filterMaxUsd}
			onchange={applyFilters}
			aria-label="Maximum USD value"
		/>

		<div class="page-size-control" role="group" aria-label="Page size">
			{#each TABLE_PAGE_SIZES as size}
				<button
					class="page-size-btn"
					class:active={pageSize === size}
					type="button"
					onclick={() => setPageSize(size)}
				>
					{size}
				</button>
			{/each}
		</div>
	</div>

	<!-- Table -->
	{#if loading && transactions.length === 0}
		<div class="loading-state">Loading transactions...</div>
	{:else if transactions.length === 0}
		<div class="empty-state">No transactions found.</div>
	{:else}
		<div class="table-wrapper">
			<div class="table-scroll">
				<table class="table">
					<thead>
						<tr>
							<th class="col-timestamp">Timestamp</th>
							<th class="col-address">Address</th>
							<th class="col-chain">Chain</th>
							<th class="col-token">Token</th>
							<th class="col-amount">Amount</th>
							<th class="col-usd">USD Value</th>
							<th class="col-tier">Tier</th>
							<th class="col-points">Points</th>
							<th class="col-txhash">TX Hash</th>
							<th class="col-watch">Watch</th>
							<th class="col-status">Status</th>
						</tr>
					</thead>
					<tbody>
						{#each transactions as tx}
							<tr>
								<td class="col-timestamp text-muted text-sm">
									{formatTimestamp(tx.detected_at)}
								</td>
								<td class="col-address">
									<span class="mono truncate" title={tx.address}>
										{truncateAddress(tx.address, 6)}
									</span>
								</td>
								<td class="col-chain">
									<span class={chainBadgeClass(tx.chain)}>{tx.chain}</span>
								</td>
								<td class="col-token text-sm text-secondary">{tx.token}</td>
								<td class="col-amount mono">{tx.amount_human}</td>
								<td class="col-usd mono">{formatUsd(tx.usd_value)}</td>
								<td class="col-tier">
									<span class="tier-badge">{tx.tier}</span>
								</td>
								<td class="col-points">
									<span class="points-value">{formatPoints(tx.points)}</span>
								</td>
								<td class="col-txhash">
									<a
										href={getTxExplorerUrl(tx.chain, tx.tx_hash, network)}
										target="_blank"
										rel="noopener noreferrer"
										class="mono text-accent text-sm"
										title={tx.tx_hash}
									>
										{truncateAddress(tx.tx_hash, 5)}
									</a>
								</td>
								<td class="col-watch">
									<span class="text-sm text-muted mono">
										{tx.watch_id.length > 8
											? tx.watch_id.slice(0, 8)
											: tx.watch_id}
									</span>
								</td>
								<td class="col-status">
									<span
										class="badge"
										class:badge-confirmed={tx.status === 'CONFIRMED'}
										class:badge-pending={tx.status === 'PENDING'}
									>
										{tx.status}
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Pagination -->
			{#if meta && meta.total > 0}
				<div class="pagination">
					<span class="pagination-info">
						Showing {showFrom}–{showTo} of {meta.total} transactions
					</span>
					<div class="pagination-controls">
						{#if currentPage > 1}
							<button
								class="pagination-btn"
								type="button"
								aria-label="Previous page"
								onclick={() => goToPage(currentPage - 1)}
							>
								<svg
									width="14"
									height="14"
									viewBox="0 0 14 14"
									fill="none"
									stroke="currentColor"
									stroke-width="1.5"
									stroke-linecap="round"
									stroke-linejoin="round"
								>
									<polyline points="9,2 4,7 9,12" />
								</svg>
							</button>
						{/if}
						{#each pageNumbers as page}
							<button
								class="pagination-btn"
								class:active={page === currentPage}
								type="button"
								aria-label="Page {page}"
								aria-current={page === currentPage ? 'page' : undefined}
								onclick={() => goToPage(page)}
							>
								{page}
							</button>
						{/each}
						{#if currentPage < totalPages}
							<button
								class="pagination-btn"
								type="button"
								aria-label="Next page"
								onclick={() => goToPage(currentPage + 1)}
							>
								<svg
									width="14"
									height="14"
									viewBox="0 0 14 14"
									fill="none"
									stroke="currentColor"
									stroke-width="1.5"
									stroke-linecap="round"
									stroke-linejoin="round"
								>
									<polyline points="5,2 10,7 5,12" />
								</svg>
							</button>
						{/if}
					</div>
				</div>
			{/if}
		</div>
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

	/* Filter Bar */
	.filter-bar {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
		margin-bottom: 1.25rem;
	}

	.form-select {
		appearance: none;
		background: var(--color-bg-input);
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		color: var(--color-text-primary);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		padding: 0.5rem 2rem 0.5rem 0.75rem;
		min-width: 130px;
		cursor: pointer;
		background-image: url("data:image/svg+xml,%3Csvg width='10' height='6' viewBox='0 0 10 6' fill='none' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M1 1l4 4 4-4' stroke='%236b7280' stroke-width='1.5' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 0.75rem center;
	}

	.form-select:focus {
		border-color: var(--color-border-focus);
		outline: none;
	}

	.form-input {
		background: var(--color-bg-input);
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		color: var(--color-text-primary);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		padding: 0.5rem 0.75rem;
	}

	.form-input:focus {
		border-color: var(--color-border-focus);
		outline: none;
	}

	.form-input-narrow {
		width: 90px;
	}

	.form-input::placeholder {
		color: var(--color-text-muted);
	}

	/* Hide number input spinners */
	.form-input[type='number']::-webkit-outer-spin-button,
	.form-input[type='number']::-webkit-inner-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}

	.form-input[type='number'] {
		-moz-appearance: textfield;
	}

	.toolbar-separator {
		width: 1px;
		height: 24px;
		background: var(--color-border-default);
		margin: 0 0.25rem;
	}

	/* Page Size Control */
	.page-size-control {
		display: flex;
		align-items: center;
		background: var(--color-bg-input);
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		overflow: hidden;
		margin-left: auto;
	}

	.page-size-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0 0.75rem;
		height: 36px;
		background: transparent;
		border: none;
		border-right: 1px solid var(--color-border-default);
		color: var(--color-text-muted);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.page-size-btn:last-child {
		border-right: none;
	}

	.page-size-btn:hover {
		color: var(--color-text-primary);
		background: var(--color-bg-surface-hover);
	}

	.page-size-btn.active {
		background: var(--color-accent-muted);
		color: var(--color-accent-text);
	}

	/* Table */
	.table-wrapper {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.table-scroll {
		overflow-x: auto;
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

	/* Column styles */
	.col-timestamp {
		white-space: nowrap;
		min-width: 130px;
	}

	.col-address {
		min-width: 140px;
		max-width: 160px;
	}

	.col-chain {
		min-width: 70px;
	}

	.col-token {
		min-width: 70px;
	}

	.col-amount {
		min-width: 100px;
		text-align: right;
	}

	.col-usd {
		min-width: 90px;
		text-align: right;
	}

	.col-tier {
		min-width: 55px;
		text-align: center;
	}

	.col-points {
		min-width: 80px;
		text-align: right;
	}

	.col-txhash {
		min-width: 120px;
		max-width: 140px;
	}

	.col-watch {
		min-width: 70px;
	}

	.col-status {
		min-width: 100px;
	}

	/* Text helpers */
	.text-muted {
		color: var(--color-text-muted);
	}

	.text-secondary {
		color: var(--color-text-secondary);
	}

	.text-sm {
		font-size: 0.8125rem;
	}

	.text-accent {
		color: var(--color-accent-text);
	}

	.text-accent:hover {
		color: var(--color-accent-hover);
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.truncate {
		display: block;
		max-width: 150px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* Badge */
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

	.badge-confirmed {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.badge-pending {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	/* Tier badge */
	.tier-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border-radius: var(--radius-md);
		background: var(--color-bg-surface-active);
		font-size: 0.6875rem;
		font-weight: 600;
		color: var(--color-text-secondary);
		font-family: var(--font-mono);
	}

	/* Points value */
	.points-value {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-accent-text);
	}

	/* Pagination */
	.pagination {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.75rem 1rem;
		border-top: 1px solid var(--color-border-subtle);
	}

	.pagination-info {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
	}

	.pagination-controls {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.pagination-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 32px;
		height: 32px;
		padding: 0 0.5rem;
		background: transparent;
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		color: var(--color-text-secondary);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.pagination-btn:hover {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-primary);
	}

	.pagination-btn.active {
		background: var(--color-accent-muted);
		border-color: var(--color-accent-default);
		color: var(--color-accent-text);
	}
</style>
