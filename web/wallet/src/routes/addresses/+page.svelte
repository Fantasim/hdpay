<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import AddressTable from '$lib/components/address/AddressTable.svelte';
	import { addressStore } from '$lib/stores/addresses.svelte';
	import { exportAddresses } from '$lib/utils/api';
	import { formatNumber } from '$lib/utils/formatting';
	import { SUPPORTED_CHAINS, CHAIN_COLORS } from '$lib/constants';
	import type { Chain } from '$lib/types';

	const store = addressStore;

	type FilterType = 'all' | 'hasBalance' | 'NATIVE' | 'USDC' | 'USDT';

	let activeFilter: FilterType = $state('all');

	const chainTabs: Array<{ label: string; value: Chain }> = SUPPORTED_CHAINS.map((c) => ({
		label: c,
		value: c as Chain
	}));

	let activeChainTab: Chain = $state('BTC');

	const filters: Array<{ label: string; value: FilterType }> = [
		{ label: 'All', value: 'all' },
		{ label: 'Has Balance', value: 'hasBalance' },
		{ label: 'Native', value: 'NATIVE' },
		{ label: 'USDC', value: 'USDC' },
		{ label: 'USDT', value: 'USDT' }
	];

	function handleChainTab(value: Chain): void {
		activeChainTab = value;
		store.setChain(value);
	}

	function handleFilter(filter: FilterType): void {
		activeFilter = filter;
		switch (filter) {
			case 'all':
				store.setFilter({ hasBalance: false, token: '' });
				break;
			case 'hasBalance':
				store.setFilter({ hasBalance: true, token: '' });
				break;
			case 'NATIVE':
				store.setFilter({ hasBalance: false, token: 'NATIVE' });
				break;
			case 'USDC':
				store.setFilter({ hasBalance: false, token: 'USDC' });
				break;
			case 'USDT':
				store.setFilter({ hasBalance: false, token: 'USDT' });
				break;
		}
	}

	function handleExport(): void {
		exportAddresses(store.state.chain);
	}

	// Pagination helpers
	let totalPages = $derived(
		store.state.meta?.total && store.state.meta?.pageSize
			? Math.ceil(store.state.meta.total / store.state.meta.pageSize)
			: 0
	);

	function pageNumbers(): number[] {
		const current = store.state.page;
		const total = totalPages;
		if (total <= 7) {
			return Array.from({ length: total }, (_, i) => i + 1);
		}

		const pages: number[] = [1];
		const start = Math.max(2, current - 1);
		const end = Math.min(total - 1, current + 1);

		if (start > 2) pages.push(-1); // ellipsis
		for (let i = start; i <= end; i++) pages.push(i);
		if (end < total - 1) pages.push(-1); // ellipsis
		pages.push(total);

		return pages;
	}

	let paginationInfo = $derived(
		store.state.meta?.total !== undefined
			? `Showing ${((store.state.page - 1) * store.state.pageSize) + 1} \u2014 ${Math.min(store.state.page * store.state.pageSize, store.state.meta.total)} of ${formatNumber(store.state.meta.total)}`
			: ''
	);

	onMount(() => {
		store.fetchAddresses();
	});
</script>

<Header title="Addresses">
	{#snippet actions()}
		<button class="btn btn-secondary" onclick={handleExport}>
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
				<path d="M8 2v8M4 6l4 4 4-4"/>
				<path d="M2 12h12v2H2z"/>
			</svg>
			Export JSON
		</button>
	{/snippet}
</Header>

<!-- Chain Tabs -->
<div class="chain-tabs">
	{#each chainTabs as tab}
		<button
			class="tab"
			class:active={activeChainTab === tab.value}
			onclick={() => handleChainTab(tab.value)}
		>
			<span
				class="chain-dot"
				style="background: {CHAIN_COLORS[tab.value]}"
			></span>
			{tab.label}
		</button>
	{/each}
</div>

<!-- Filter Toolbar -->
<div class="filter-toolbar">
	{#each filters as filter}
		<button
			class="filter-chip"
			class:active={activeFilter === filter.value}
			onclick={() => handleFilter(filter.value)}
		>
			{filter.label}
		</button>
	{/each}
</div>

<!-- Address Table -->
<AddressTable
	addresses={store.state.addresses}
	loading={store.state.loading}
/>

<!-- Error Display -->
{#if store.state.error}
	<div class="error-banner">
		{store.state.error}
	</div>
{/if}

<!-- Pagination -->
{#if totalPages > 0}
	<div class="pagination">
		<span class="pagination-info">{paginationInfo}</span>
		<div class="pagination-controls">
			<button
				class="pagination-btn"
				disabled={store.state.page <= 1}
				onclick={() => store.setPage(1)}
			>&laquo;</button>
			<button
				class="pagination-btn"
				disabled={store.state.page <= 1}
				onclick={() => store.setPage(store.state.page - 1)}
			>&lsaquo;</button>
			{#each pageNumbers() as pn}
				{#if pn === -1}
					<span class="pagination-ellipsis">&hellip;</span>
				{:else}
					<button
						class="pagination-btn"
						class:active={store.state.page === pn}
						onclick={() => store.setPage(pn)}
					>{pn}</button>
				{/if}
			{/each}
			<button
				class="pagination-btn"
				disabled={store.state.page >= totalPages}
				onclick={() => store.setPage(store.state.page + 1)}
			>&rsaquo;</button>
			<button
				class="pagination-btn"
				disabled={store.state.page >= totalPages}
				onclick={() => store.setPage(totalPages)}
			>&raquo;</button>
		</div>
	</div>
{/if}

<style>
	/* Chain Tabs */
	.chain-tabs {
		display: flex;
		gap: 0;
		border-bottom: 1px solid var(--color-border);
		margin-bottom: 1rem;
	}

	.tab {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.625rem 1rem;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-muted);
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		transition: all 150ms ease;
	}

	.tab:hover {
		color: var(--color-text-primary);
	}

	.tab.active {
		color: var(--color-text-primary);
		border-bottom-color: var(--color-accent);
	}

	.chain-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	/* Filter Toolbar */
	.filter-toolbar {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 1rem;
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

	/* Button */
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 150ms ease;
		border: none;
	}

	.btn-secondary {
		background: var(--color-bg-surface);
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
	}

	.btn-secondary:hover {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-primary);
		border-color: var(--color-border-hover);
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

	/* Error */
	.error-banner {
		margin-top: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		background: var(--color-error-muted);
		color: var(--color-error);
		font-size: 0.8125rem;
	}
</style>
