<script lang="ts">
	import { Chart } from 'svelte-echarts';
	import { init, use } from 'echarts/core';
	import { PieChart } from 'echarts/charts';
	import { TooltipComponent, LegendComponent } from 'echarts/components';
	import { CanvasRenderer } from 'echarts/renderers';
	import type { EChartsOption } from 'echarts';
	import type { ChainPortfolio } from '$lib/types';
	import { CHAIN_COLORS } from '$lib/constants';
	import { formatUsd } from '$lib/utils/formatting';

	// Register ECharts components (tree-shaking).
	use([PieChart, TooltipComponent, LegendComponent, CanvasRenderer]);

	interface Props {
		chains: ChainPortfolio[];
	}

	let { chains }: Props = $props();

	// Aggregate USD per chain for the pie chart.
	let chartData = $derived(
		chains
			.map((c) => {
				const totalUsd = c.tokens.reduce((sum, t) => sum + t.usd, 0);
				return { name: c.chain, value: Math.round(totalUsd * 100) / 100 };
			})
			.filter((d) => d.value > 0)
	);

	let hasData = $derived(chartData.length > 0);

	let options = $derived<EChartsOption>({
		tooltip: {
			trigger: 'item',
			backgroundColor: '#1a1d27',
			borderColor: '#2a2d3a',
			textStyle: { color: '#e8eaed', fontSize: 13 },
			formatter: (params: Record<string, unknown>) => {
				const p = params as { name: string; value: number; percent: number };
				return `${p.name}<br/>${formatUsd(p.value)} (${p.percent}%)`;
			}
		},
		legend: {
			orient: 'vertical',
			right: '5%',
			top: 'center',
			textStyle: { color: '#9ba1b0', fontSize: 13 },
			itemWidth: 12,
			itemHeight: 12,
			itemGap: 16
		},
		series: [
			{
				type: 'pie',
				radius: ['40%', '70%'],
				center: ['35%', '50%'],
				avoidLabelOverlap: true,
				itemStyle: {
					borderRadius: 6,
					borderColor: '#0f1117',
					borderWidth: 2
				},
				label: { show: false },
				emphasis: {
					label: { show: false }
				},
				data: chartData.map((d) => ({
					name: d.name,
					value: d.value,
					itemStyle: {
						color: CHAIN_COLORS[d.name as keyof typeof CHAIN_COLORS] ?? '#3b82f6'
					}
				}))
			}
		]
	});
</script>

<div class="section">
	<div class="section-title">Portfolio Distribution</div>
	<div class="card">
		{#if hasData}
			<div class="chart-container">
				<Chart {init} {options} />
			</div>
		{:else}
			<div class="chart-placeholder">
				No balance data — run a scan to see distribution
			</div>
		{/if}
	</div>
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

	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: 1.5rem;
	}

	.chart-container {
		height: 280px;
		width: 100%;
	}

	.chart-placeholder {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 240px;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
