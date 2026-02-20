<script lang="ts">
	import ChartWrapper from './ChartWrapper.svelte';
	import { CHART_COLORS } from '$lib/constants';
	import { formatUsd } from '$lib/utils/formatting';
	import type { EChartsOption } from 'echarts';

	interface Props {
		data: Array<{ date: string; usd: number }>;
	}

	let { data }: Props = $props();

	let options: EChartsOption = $derived({
		backgroundColor: 'transparent',
		tooltip: {
			trigger: 'axis',
			formatter: (params: unknown) => {
				const p = (params as Array<{ name: string; value: number }>)[0];
				return `${p.name}<br/>${formatUsd(p.value)}`;
			}
		},
		grid: { left: 60, right: 16, top: 16, bottom: 32 },
		xAxis: {
			type: 'category',
			data: data.map((d) => d.date),
			axisLine: { lineStyle: { color: '#3f3f46' } },
			axisLabel: { color: '#a1a1aa', fontSize: 11 }
		},
		yAxis: {
			type: 'value',
			axisLabel: {
				color: '#a1a1aa',
				fontSize: 11,
				formatter: (v: number) => `$${v >= 1000 ? `${(v / 1000).toFixed(0)}k` : v}`
			},
			splitLine: { lineStyle: { color: '#27272a' } }
		},
		series: [
			{
				type: 'bar',
				data: data.map((d) => d.usd),
				itemStyle: { color: CHART_COLORS[0], borderRadius: [2, 2, 0, 0] },
				barMaxWidth: 32
			}
		]
	});
</script>

<ChartWrapper {options} />
