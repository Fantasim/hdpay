<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import PortfolioOverview from '$lib/components/dashboard/PortfolioOverview.svelte';
	import BalanceBreakdown from '$lib/components/dashboard/BalanceBreakdown.svelte';
	import PortfolioCharts from '$lib/components/dashboard/PortfolioCharts.svelte';
	import { getPortfolio } from '$lib/utils/api';
	import { PORTFOLIO_REFRESH_INTERVAL_MS } from '$lib/constants';
	import type { PortfolioResponse } from '$lib/types';

	let portfolio = $state<PortfolioResponse | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	async function fetchPortfolio(): Promise<void> {
		try {
			const res = await getPortfolio();
			portfolio = res.data;
			error = null;
		} catch (err) {
			if (!portfolio) {
				error = err instanceof Error ? err.message : 'Failed to load portfolio';
			}
			console.error('Portfolio fetch error:', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		fetchPortfolio();

		const interval = setInterval(fetchPortfolio, PORTFOLIO_REFRESH_INTERVAL_MS);

		return () => {
			clearInterval(interval);
		};
	});
</script>

<Header title="Dashboard" />

{#if error && !portfolio}
	<div class="error-banner">
		{error}
	</div>
{/if}

<PortfolioOverview {portfolio} {loading} />

<BalanceBreakdown chains={portfolio?.chains ?? []} />

<PortfolioCharts chains={portfolio?.chains ?? []} />

<style>
	.error-banner {
		margin-bottom: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		background: var(--color-error-muted);
		color: var(--color-error);
		font-size: 0.8125rem;
	}
</style>
