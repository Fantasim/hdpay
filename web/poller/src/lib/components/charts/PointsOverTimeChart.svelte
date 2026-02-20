<script lang="ts">
	import ChartWrapper from './ChartWrapper.svelte';
	import { CHART_COLORS } from '$lib/constants';
	import { formatPoints } from '$lib/utils/formatting';
	import type { EChartsOption } from 'echarts';

	interface Props {
		data: Array<{ date: string; points: number }>;
	}

	let { data }: Props = $props();

	let options: EChartsOption = $derived({
		backgroundColor: 'transparent',
		tooltip: {
			trigger: 'axis',
			formatter: (params: unknown) => {
				const p = (params as Array<{ name: string; value: number }>)[0];
				return `${p.name}<br/>${formatPoints(p.value)} pts`;
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
				formatter: (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(0)}k` : String(v))
			},
			splitLine: { lineStyle: { color: '#27272a' } }
		},
		series: [
			{
				type: 'line',
				data: data.map((d) => d.points),
				itemStyle: { color: CHART_COLORS[0] },
				lineStyle: { color: CHART_COLORS[0], width: 2 },
				smooth: true,
				symbol: 'circle',
				symbolSize: 6,
				areaStyle: { color: 'rgba(247, 147, 26, 0.08)' }
			}
		]
	});
</script>

<ChartWrapper {options} />
