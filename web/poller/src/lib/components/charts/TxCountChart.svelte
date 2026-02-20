<script lang="ts">
	import ChartWrapper from './ChartWrapper.svelte';
	import type { EChartsOption } from 'echarts';

	interface Props {
		data: Array<{ date: string; count: number }>;
	}

	let { data }: Props = $props();

	let options: EChartsOption = $derived({
		backgroundColor: 'transparent',
		tooltip: {
			trigger: 'axis',
			formatter: (params: unknown) => {
				const p = (params as Array<{ name: string; value: number }>)[0];
				return `${p.name}<br/>${p.value} transactions`;
			}
		},
		grid: { left: 48, right: 16, top: 16, bottom: 32 },
		xAxis: {
			type: 'category',
			data: data.map((d) => d.date),
			axisLine: { lineStyle: { color: '#3f3f46' } },
			axisLabel: { color: '#a1a1aa', fontSize: 11 }
		},
		yAxis: {
			type: 'value',
			axisLabel: { color: '#a1a1aa', fontSize: 11 },
			splitLine: { lineStyle: { color: '#27272a' } }
		},
		series: [
			{
				type: 'bar',
				data: data.map((d) => d.count),
				itemStyle: { color: '#3b82f6', borderRadius: [2, 2, 0, 0] },
				barMaxWidth: 32
			}
		]
	});
</script>

<ChartWrapper {options} />
