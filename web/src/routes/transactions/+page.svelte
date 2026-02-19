<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { getTransactions } from '$lib/utils/api';
	import { truncateAddress, formatRawBalance, copyToClipboard } from '$lib/utils/formatting';
	import { getExplorerTxUrl } from '$lib/utils/chains';
	import { SUPPORTED_CHAINS, DEFAULT_TX_PAGE_SIZE } from '$lib/constants';
	import type { Chain, Transaction, TransactionDirection, TransactionStatus, TransactionListParams } from '$lib/types';

	// Filter state
	let chainFilter: Chain | null = $state(null);
	let directionFilter: TransactionDirection | null = $state(null);
	let tokenFilter: string | null = $state(null);

	// Data state
	let transactions: Transaction[] = $state([]);
	let loading = $state(true);
	let error: string | null = $state(null);
	let page = $state(1);
	let total = $state(0);
	let copiedHash: string | null = $state(null);

	async function fetchTransactions(): Promise<void> {
		loading = true;
		error = null;

		try {
			const params: TransactionListParams = {
				page,
				pageSize: DEFAULT_TX_PAGE_SIZE
			};
			if (chainFilter) params.chain = chainFilter;
			if (directionFilter) params.direction = directionFilter;
			if (tokenFilter) params.token = tokenFilter;

			const res = await getTransactions(params);
			transactions = res.data ?? [];
			total = res.meta?.total ?? 0;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to fetch transactions';
			transactions = [];
			total = 0;
		} finally {
			loading = false;
		}
	}

	function setChainFilter(chain: Chain | null): void {
		chainFilter = chain;
		page = 1;
		fetchTransactions();
	}

	function setDirectionFilter(dir: TransactionDirection | null): void {
		directionFilter = dir;
		page = 1;
		fetchTransactions();
	}

	function setTokenFilter(token: string | null): void {
		tokenFilter = token;
		page = 1;
		fetchTransactions();
	}

	function setPage(p: number): void {
		page = p;
		fetchTransactions();
	}

	async function handleCopy(hash: string): Promise<void> {
		const ok = await copyToClipboard(hash);
		if (ok) {
			copiedHash = hash;
			setTimeout(() => { copiedHash = null; }, 1500);
		}
	}

	function getTokenDisplay(tx: Transaction): string {
		if (tx.token === 'NATIVE') {
			const nativeMap: Record<string, string> = { BTC: 'BTC', BSC: 'BNB', SOL: 'SOL' };
			return nativeMap[tx.chain] ?? tx.token;
		}
		return tx.token;
	}

	function formatTxDate(dateStr: string): { date: string; time: string } {
		const d = new Date(dateStr + 'Z');
		return {
			date: d.toLocaleDateString('en-CA'), // YYYY-MM-DD
			time: d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
		};
	}

	function statusClass(status: TransactionStatus): string {
		switch (status) {
			case 'confirmed': return 'badge-success';
			case 'pending': return 'badge-warning';
			case 'failed': return 'badge-error';
			default: return '';
		}
	}

	// Pagination
	let totalPages = $derived(Math.ceil(total / DEFAULT_TX_PAGE_SIZE));

	let paginationInfo = $derived(
		total > 0
			? `Showing ${(page - 1) * DEFAULT_TX_PAGE_SIZE + 1} \u2014 ${Math.min(page * DEFAULT_TX_PAGE_SIZE, total)} of ${total}`
			: ''
	);

	function pageNumbers(): number[] {
		if (totalPages <= 7) {
			return Array.from({ length: totalPages }, (_, i) => i + 1);
		}
		const pages: number[] = [1];
		const start = Math.max(2, page - 1);
		const end = Math.min(totalPages - 1, page + 1);
		if (start > 2) pages.push(-1);
		for (let i = start; i <= end; i++) pages.push(i);
		if (end < totalPages - 1) pages.push(-1);
		pages.push(totalPages);
		return pages;
	}

	onMount(() => {
		fetchTransactions();
	});
</script>

<Header title="Transactions" />
<p class="page-subtitle">View all transaction history</p>

<!-- Filter Toolbar -->
<div class="filter-toolbar">
	<!-- Chain filter -->
	<div class="filter-group">
		<span class="filter-group-label">Chain</span>
		<button class="filter-chip" class:active={chainFilter === null} onclick={() => setChainFilter(null)}>All</button>
		{#each SUPPORTED_CHAINS as chain}
			<button class="filter-chip" class:active={chainFilter === chain} onclick={() => setChainFilter(chain)}>{chain}</button>
		{/each}
	</div>

	<span class="toolbar-separator"></span>

	<!-- Direction filter -->
	<div class="filter-group">
		<span class="filter-group-label">Direction</span>
		<button class="filter-chip" class:active={directionFilter === null} onclick={() => setDirectionFilter(null)}>All</button>
		<button class="filter-chip" class:active={directionFilter === 'in'} onclick={() => setDirectionFilter('in')}>Incoming</button>
		<button class="filter-chip" class:active={directionFilter === 'out'} onclick={() => setDirectionFilter('out')}>Outgoing</button>
	</div>

	<span class="toolbar-separator"></span>

	<!-- Token filter -->
	<div class="filter-group">
		<span class="filter-group-label">Token</span>
		<button class="filter-chip" class:active={tokenFilter === null} onclick={() => setTokenFilter(null)}>All</button>
		<button class="filter-chip" class:active={tokenFilter === 'NATIVE'} onclick={() => setTokenFilter('NATIVE')}>Native</button>
		<button class="filter-chip" class:active={tokenFilter === 'USDC'} onclick={() => setTokenFilter('USDC')}>USDC</button>
		<button class="filter-chip" class:active={tokenFilter === 'USDT'} onclick={() => setTokenFilter('USDT')}>USDT</button>
	</div>
</div>

<!-- Error -->
{#if error}
	<div class="error-banner">{error}</div>
{/if}

<!-- Loading -->
{#if loading}
	<div class="loading-state">Loading transactions...</div>
{:else if transactions.length === 0}
	<div class="empty-state">
		<p>No transactions found</p>
		<p class="text-muted">Transactions will appear here after you send or receive funds.</p>
	</div>
{:else}
	<!-- Table -->
	<div class="table-wrapper">
		<table class="table">
			<thead>
				<tr>
					<th>Date</th>
					<th>Chain</th>
					<th>Direction</th>
					<th>Token</th>
					<th class="text-right">Amount</th>
					<th>From / To</th>
					<th>Tx Hash</th>
					<th>Status</th>
				</tr>
			</thead>
			<tbody>
				{#each transactions as tx (tx.id)}
					{@const dt = formatTxDate(tx.createdAt)}
					<tr>
						<td class="date-cell">
							<div class="date-primary">{dt.date}</div>
							<div class="date-secondary">{dt.time}</div>
						</td>
						<td><span class="badge badge-{tx.chain.toLowerCase()}">{tx.chain}</span></td>
						<td>
							<div class="direction-cell">
								<span class="direction-icon" class:direction-icon-incoming={tx.direction === 'in'} class:direction-icon-outgoing={tx.direction === 'out'}>
									{#if tx.direction === 'in'}
										<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
											<path d="M9 3L3 9M3 9V4M3 9h5"/>
										</svg>
									{:else}
										<svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
											<path d="M3 9L9 3M9 3v5M9 3H4"/>
										</svg>
									{/if}
								</span>
								<span class="direction-label" class:direction-label-incoming={tx.direction === 'in'} class:direction-label-outgoing={tx.direction === 'out'}>
									{tx.direction === 'in' ? 'Incoming' : 'Outgoing'}
								</span>
							</div>
						</td>
						<td>{getTokenDisplay(tx)}</td>
						<td class="mono text-right">{formatRawBalance(tx.amount, tx.chain as Chain, tx.token)}</td>
						<td class="mono text-sm">
							{#if tx.direction === 'out'}
								{truncateAddress(tx.toAddress)}
							{:else}
								{truncateAddress(tx.fromAddress)}
							{/if}
						</td>
						<td>
							<div class="tx-hash-cell">
								{#if tx.txHash}
									<a href={getExplorerTxUrl(tx.chain, tx.txHash, 'testnet')} target="_blank" rel="noopener" class="mono text-sm tx-hash-link">
										{truncateAddress(tx.txHash)}
									</a>
									<button class="copy-btn" title={copiedHash === tx.txHash ? 'Copied!' : 'Copy tx hash'} onclick={() => handleCopy(tx.txHash)}>
										{#if copiedHash === tx.txHash}
											<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
												<path d="M3 8.5l3 3 7-7"/>
											</svg>
										{:else}
											<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
												<rect x="5" y="5" width="9" height="9" rx="1"/>
												<path d="M11 5V3a1 1 0 0 0-1-1H3a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h2"/>
											</svg>
										{/if}
									</button>
								{:else}
									<span class="mono text-sm text-muted">&mdash;</span>
								{/if}
							</div>
						</td>
						<td><span class="badge {statusClass(tx.status)}">{tx.status}</span></td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<!-- Pagination -->
	{#if totalPages > 1}
		<div class="pagination">
			<span class="pagination-info">{paginationInfo}</span>
			<div class="pagination-controls">
				<button class="pagination-btn" disabled={page <= 1} onclick={() => setPage(1)}>&laquo;</button>
				<button class="pagination-btn" disabled={page <= 1} onclick={() => setPage(page - 1)}>&lsaquo;</button>
				{#each pageNumbers() as pn}
					{#if pn === -1}
						<span class="pagination-ellipsis">&hellip;</span>
					{:else}
						<button class="pagination-btn" class:active={page === pn} onclick={() => setPage(pn)}>{pn}</button>
					{/if}
				{/each}
				<button class="pagination-btn" disabled={page >= totalPages} onclick={() => setPage(page + 1)}>&rsaquo;</button>
				<button class="pagination-btn" disabled={page >= totalPages} onclick={() => setPage(totalPages)}>&raquo;</button>
			</div>
		</div>
	{/if}
{/if}

<style>
	.page-subtitle {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		margin: -1rem 0 1.5rem 0;
	}

	/* Filter Toolbar */
	.filter-toolbar {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		padding-bottom: 1rem;
		flex-wrap: wrap;
	}

	.filter-group {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.filter-group-label {
		font-size: 0.6875rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-right: 0.125rem;
	}

	.toolbar-separator {
		width: 1px;
		height: 20px;
		background: var(--color-border);
		margin: 0 0.25rem;
	}

	.filter-chip {
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.75rem;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border);
		cursor: pointer;
		transition: all 150ms ease;
	}

	.filter-chip:hover {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-primary);
	}

	.filter-chip.active {
		background: var(--color-accent-muted);
		color: var(--color-accent-text);
		border-color: transparent;
	}

	/* Table */
	.table-wrapper {
		overflow-x: auto;
		border: 1px solid var(--color-border);
		border-radius: 8px;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8125rem;
	}

	.table thead {
		background: var(--color-bg-surface);
	}

	.table th {
		padding: 0.625rem 0.75rem;
		text-align: left;
		font-size: 0.6875rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		border-bottom: 1px solid var(--color-border);
		white-space: nowrap;
	}

	.table td {
		padding: 0.625rem 0.75rem;
		color: var(--color-text-primary);
		border-bottom: 1px solid var(--color-border-subtle);
		vertical-align: middle;
	}

	.table tbody tr:last-child td {
		border-bottom: none;
	}

	.table tbody tr:hover {
		background: var(--color-bg-surface-hover);
	}

	.text-right { text-align: right; }
	.text-sm { font-size: 0.75rem; }
	.text-muted { color: var(--color-text-muted); }
	.mono { font-family: 'JetBrains Mono', monospace; }

	/* Chain badges */
	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 9999px;
		font-size: 0.6875rem;
		font-weight: 600;
		letter-spacing: 0.02em;
	}

	.badge-btc { background: #f7931a20; color: #f7931a; }
	.badge-bsc { background: #F0B90B20; color: #F0B90B; }
	.badge-sol { background: #9945FF20; color: #9945FF; }
	.badge-success { background: var(--color-success-muted); color: var(--color-success); }
	.badge-warning { background: var(--color-warning-muted); color: var(--color-warning); }
	.badge-error { background: var(--color-error-muted); color: var(--color-error); }

	/* Direction */
	.direction-cell {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.direction-icon {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.direction-icon-incoming {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.direction-icon-outgoing {
		background: var(--color-info-muted, #3b82f620);
		color: var(--color-info, #3b82f6);
	}

	.direction-label {
		font-size: 0.8125rem;
		font-weight: 500;
	}

	.direction-label-incoming { color: var(--color-success); }
	.direction-label-outgoing { color: var(--color-info, #3b82f6); }

	/* Date */
	.date-cell { white-space: nowrap; }
	.date-primary { font-size: 0.8125rem; color: var(--color-text-primary); }
	.date-secondary { font-size: 0.6875rem; color: var(--color-text-muted); }

	/* Tx Hash */
	.tx-hash-cell {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.tx-hash-link {
		color: var(--color-text-secondary);
		text-decoration: none;
	}

	.tx-hash-link:hover {
		color: var(--color-accent);
		text-decoration: underline;
	}

	.copy-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.125rem;
		background: none;
		border: none;
		color: var(--color-text-muted);
		cursor: pointer;
		border-radius: 4px;
		transition: all 150ms ease;
	}

	.copy-btn:hover {
		color: var(--color-text-primary);
		background: var(--color-bg-surface-hover);
	}

	/* States */
	.loading-state, .empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 300px;
		border: 1px dashed var(--color-border);
		border-radius: 8px;
		color: var(--color-text-muted);
		font-size: 0.875rem;
		gap: 0.5rem;
	}

	.error-banner {
		margin-bottom: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		background: var(--color-error-muted);
		color: var(--color-error);
		font-size: 0.8125rem;
	}

	/* Pagination */
	.pagination {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.75rem 1rem;
		margin-top: 0.5rem;
	}

	.pagination-info {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.pagination-controls {
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.pagination-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 32px;
		height: 32px;
		padding: 0 0.375rem;
		border-radius: 6px;
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-secondary);
		background: transparent;
		border: 1px solid var(--color-border);
		cursor: pointer;
		transition: all 150ms ease;
	}

	.pagination-btn:hover:not(:disabled) {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-primary);
	}

	.pagination-btn.active {
		background: var(--color-accent);
		color: white;
		border-color: var(--color-accent);
	}

	.pagination-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.pagination-ellipsis {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}
</style>
