<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import TimeRangeSelector from '$lib/components/dashboard/TimeRangeSelector.svelte';
	import StatsCard from '$lib/components/dashboard/StatsCard.svelte';
	import UsdOverTimeChart from '$lib/components/charts/UsdOverTimeChart.svelte';
	import PointsOverTimeChart from '$lib/components/charts/PointsOverTimeChart.svelte';
	import TxCountChart from '$lib/components/charts/TxCountChart.svelte';
	import ChainBreakdownChart from '$lib/components/charts/ChainBreakdownChart.svelte';
	import TokenBreakdownChart from '$lib/components/charts/TokenBreakdownChart.svelte';
	import TierDistributionChart from '$lib/components/charts/TierDistributionChart.svelte';
	import WatchesOverTimeChart from '$lib/components/charts/WatchesOverTimeChart.svelte';
	import { getDashboardStats, getDashboardCharts } from '$lib/utils/api';
	import { formatUsd, formatPoints, formatNumber } from '$lib/utils/formatting';
	import { TIME_RANGE_LABELS } from '$lib/constants';
	import type { TimeRange, DashboardStats, ChartData } from '$lib/types';

	let selectedRange: TimeRange = $state('week');
	let stats: DashboardStats | null = $state(null);
	let charts: ChartData | null = $state(null);
	let loading = $state(true);

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const [statsRes, chartsRes] = await Promise.all([
				getDashboardStats(selectedRange),
				getDashboardCharts(selectedRange)
			]);
			stats = statsRes.data;
			charts = chartsRes.data;
		} catch (err) {
			console.error('Failed to fetch dashboard data', err);
		} finally {
			loading = false;
		}
	}

	function handleRangeChange(range: TimeRange): void {
		selectedRange = range;
		fetchData();
	}

	onMount(() => {
		fetchData();
	});

	let rangeHint = $derived(TIME_RANGE_LABELS[selectedRange].toLowerCase());
</script>

<svelte:head>
	<title>Poller — Overview</title>
</svelte:head>

<Header title="Overview">
	{#snippet actions()}
		<TimeRangeSelector selected={selectedRange} onchange={handleRangeChange} />
	{/snippet}
</Header>

<div class="page-body">
	{#if loading && !stats}
		<div class="loading-state">Loading dashboard...</div>
	{:else if stats}
		<!-- Stats Row 1 -->
		<div class="stats-grid">
			<StatsCard
				label="Active Watches"
				value={formatNumber(stats.active_watches)}
				hint="of {formatNumber(stats.total_watches)} total"
			/>
			<StatsCard
				label="Total Watches"
				value={formatNumber(stats.total_watches)}
				hint="all time"
			/>
			<StatsCard
				label="USD Received"
				value={formatUsd(stats.usd_received)}
				hint={rangeHint}
			/>
			<StatsCard
				label="Points Awarded"
				value={formatPoints(stats.points_awarded)}
				hint={rangeHint}
			/>
		</div>

		<!-- Stats Row 2 -->
		<div class="stats-grid stats-grid-row2">
			<StatsCard
				label="Pending Points"
				value={formatPoints(stats.pending_points.total)}
				hint="across {formatNumber(stats.pending_points.accounts)} accounts"
			/>
			<StatsCard
				label="Unique Addresses"
				value={formatNumber(stats.unique_addresses)}
				hint="received payments"
			/>
			<StatsCard
				label="Avg TX Size"
				value={formatUsd(stats.avg_tx_usd)}
				hint="per transaction"
			/>
			<StatsCard
				label="Largest TX"
				value={formatUsd(stats.largest_tx_usd)}
				hint="single transaction"
			/>
		</div>

		<!-- Charts Section -->
		{#if charts}
			<div class="section-title">Charts</div>

			<div class="charts-grid">
				<div class="chart-card">
					<div class="chart-card-title">USD Received Over Time</div>
					<UsdOverTimeChart data={charts.usd_over_time} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Points Awarded Over Time</div>
					<PointsOverTimeChart data={charts.points_over_time} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Transaction Count</div>
					<TxCountChart data={charts.tx_count_over_time} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Breakdown by Chain</div>
					<ChainBreakdownChart data={charts.by_chain} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Breakdown by Token</div>
					<TokenBreakdownChart data={charts.by_token} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Tier Distribution</div>
					<TierDistributionChart data={charts.by_tier} />
				</div>

				<div class="chart-card">
					<div class="chart-card-title">Watches Over Time</div>
					<WatchesOverTimeChart data={charts.watches_over_time} />
				</div>
			</div>
		{/if}
	{/if}
</div>

<style>
	.page-body {
		padding: 1.5rem 2rem;
	}

	.loading-state {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		padding: 2rem 0;
	}

	.stats-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 1rem;
	}

	.stats-grid-row2 {
		margin-top: 1rem;
	}

	.section-title {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 1rem;
		margin-top: 1.5rem;
	}

	.charts-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 1rem;
	}

	.chart-card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		padding: 1.25rem 1.5rem;
	}

	.chart-card-title {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text-primary);
		margin-bottom: 0.75rem;
	}
</style>
