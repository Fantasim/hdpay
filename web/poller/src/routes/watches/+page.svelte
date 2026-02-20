<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { listWatches } from '$lib/utils/api';
	import { truncateAddress, formatTimestamp, formatCountdown, badgeClass } from '$lib/utils/formatting';
	import { SUPPORTED_CHAINS, WATCH_STATUSES } from '$lib/constants';
	import type { Watch, WatchStatus } from '$lib/types';

	// Filter state
	let filterStatus: WatchStatus | '' = $state('');
	let filterChain = $state('');

	// Data state
	let watches: Watch[] = $state([]);
	let loading = $state(true);

	// Countdown tick
	let tick = $state(0);

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const params: { status?: WatchStatus; chain?: string } = {};
			if (filterStatus) params.status = filterStatus;
			if (filterChain) params.chain = filterChain;
			const res = await listWatches(params);
			watches = res.data;
		} catch (err) {
			console.error('Failed to fetch watches', err);
		} finally {
			loading = false;
		}
	}

	function setStatusFilter(status: WatchStatus | ''): void {
		filterStatus = status;
		fetchData();
	}

	function setChainFilter(chain: string): void {
		filterChain = chain;
		fetchData();
	}

	function getCountdown(watch: Watch): string {
		// Reference tick to trigger reactivity
		void tick;
		if (watch.status !== 'ACTIVE') return '\u2014';
		return formatCountdown(watch.expires_at);
	}

	onMount(() => {
		fetchData();
		// Tick every second to update countdowns
		const interval = setInterval(() => {
			tick++;
		}, 1000);
		return () => clearInterval(interval);
	});
</script>

<svelte:head>
	<title>Poller — Watches</title>
</svelte:head>

<Header title="Watches" subtitle="Active and historical address watches" />

<!-- Filter Bar -->
<div class="filter-bar">
	<div class="filter-group">
		<span class="filter-group-label">Status</span>
		<div class="filter-chips">
			<button
				class="filter-chip"
				class:active={filterStatus === ''}
				type="button"
				onclick={() => setStatusFilter('')}
			>All</button>
			{#each WATCH_STATUSES as status}
				<button
					class="filter-chip"
					class:active={filterStatus === status}
					type="button"
					onclick={() => setStatusFilter(status)}
				>{status.charAt(0) + status.slice(1).toLowerCase()}</button>
			{/each}
		</div>
	</div>
	<div class="filter-group">
		<span class="filter-group-label">Chain</span>
		<div class="filter-chips">
			<button
				class="filter-chip"
				class:active={filterChain === ''}
				type="button"
				onclick={() => setChainFilter('')}
			>All</button>
			{#each SUPPORTED_CHAINS as chain}
				<button
					class="filter-chip"
					class:active={filterChain === chain}
					type="button"
					onclick={() => setChainFilter(chain)}
				>{chain}</button>
			{/each}
		</div>
	</div>
</div>

<div class="page-body">
	{#if loading && watches.length === 0}
		<div class="loading-state">Loading watches...</div>
	{:else if watches.length === 0}
		<div class="empty-state">No watches found.</div>
	{:else}
		<div class="table-wrapper">
			<div class="table-scroll">
				<table class="table">
					<thead>
						<tr>
							<th>Address</th>
							<th>Chain</th>
							<th>Status</th>
							<th>Started</th>
							<th>Time Remaining</th>
							<th>Polls</th>
							<th>Last Poll Result</th>
						</tr>
					</thead>
					<tbody>
						{#each watches as watch}
							<tr>
								<td>
									<span class="mono truncate" title={watch.address}>
										{truncateAddress(watch.address, 6)}
									</span>
								</td>
								<td>
									<span class={badgeClass(watch.chain)}>{watch.chain}</span>
								</td>
								<td>
									<span class={badgeClass(watch.status)}>{watch.status}</span>
								</td>
								<td class="text-sm">{formatTimestamp(watch.started_at)}</td>
								<td>
									{#if watch.status === 'ACTIVE'}
										<span class="countdown">{getCountdown(watch)}</span>
									{:else}
										<span class="text-muted">{'\u2014'}</span>
									{/if}
								</td>
								<td class="text-sm">{watch.poll_count}</td>
								<td class="text-sm text-secondary">
									{watch.last_poll_result ?? '\u2014'}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}
</div>

<style>
	/* Filter Bar */
	.filter-bar {
		display: flex;
		gap: 1.5rem;
		padding: 0.75rem 2rem;
		border-bottom: 1px solid var(--color-border-subtle);
		background: var(--color-bg-primary);
	}

	.filter-group {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.filter-group-label {
		font-size: 0.6875rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		white-space: nowrap;
	}

	.filter-chips {
		display: flex;
		gap: 0.375rem;
		flex-wrap: wrap;
		align-items: center;
	}

	.filter-chip {
		display: inline-flex;
		align-items: center;
		padding: 0.25rem 0.75rem;
		border-radius: var(--radius-md);
		border: 1px solid var(--color-border-default);
		background: transparent;
		color: var(--color-text-secondary);
		font-family: var(--font-sans);
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
		white-space: nowrap;
	}

	.filter-chip:hover {
		border-color: var(--color-border-hover);
		color: var(--color-text-primary);
	}

	.filter-chip.active {
		background: var(--color-accent-muted);
		border-color: var(--color-accent-default);
		color: var(--color-accent-text);
	}

	/* Page Body */
	.page-body {
		padding: 1.5rem 2rem;
	}

	.loading-state,
	.empty-state {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		padding: 2rem 0;
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

	/* Helpers */
	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.truncate {
		display: block;
		max-width: 160px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.text-sm {
		font-size: 0.8125rem;
	}

	.text-muted {
		color: var(--color-text-muted);
	}

	.text-secondary {
		color: var(--color-text-secondary);
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

	.badge-active {
		background: var(--color-info-muted);
		color: var(--color-info);
	}

	.badge-completed {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.badge-expired {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.badge-cancelled {
		background: rgba(107, 114, 128, 0.15);
		color: #9ca3af;
	}

	/* Countdown */
	.countdown {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-accent-text);
	}
</style>
