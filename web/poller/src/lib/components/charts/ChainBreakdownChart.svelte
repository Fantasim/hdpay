<script lang="ts">
	import ChartWrapper from './ChartWrapper.svelte';
	import { CHAIN_COLORS } from '$lib/constants';
	import { formatUsd } from '$lib/utils/formatting';
	import type { ChainBreakdown } from '$lib/types';
	import type { EChartsOption } from 'echarts';

	interface Props {
		data: ChainBreakdown[];
	}

	let { data }: Props = $props();

	let options: EChartsOption = $derived({
		backgroundColor: 'transparent',
		tooltip: {
			trigger: 'item',
			formatter: (params: unknown) => {
				const p = params as { name: string; value: number; percent: number };
				return `${p.name}<br/>${formatUsd(p.value)} (${p.percent.toFixed(1)}%)`;
			}
		},
		legend: {
			bottom: 0,
			textStyle: { color: '#a1a1aa', fontSize: 12 },
			itemWidth: 10,
			itemHeight: 10
		},
		series: [
			{
				type: 'pie',
				radius: ['40%', '65%'],
				center: ['50%', '42%'],
				avoidLabelOverlap: false,
				label: { show: false },
				data: data.map((d) => ({
					value: d.usd,
					name: d.chain,
					itemStyle: { color: CHAIN_COLORS[d.chain as keyof typeof CHAIN_COLORS] ?? '#6b7280' }
				}))
			}
		]
	});
</script>

<ChartWrapper {options} />
