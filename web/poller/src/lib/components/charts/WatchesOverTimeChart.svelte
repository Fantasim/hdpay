<script lang="ts">
	import ChartWrapper from './ChartWrapper.svelte';
	import { STATUS_COLORS } from '$lib/constants';
	import type { DailyWatchStat } from '$lib/types';
	import type { EChartsOption } from 'echarts';

	interface Props {
		data: DailyWatchStat[];
	}

	let { data }: Props = $props();

	let options: EChartsOption = $derived({
		backgroundColor: 'transparent',
		tooltip: {
			trigger: 'axis'
		},
		legend: {
			bottom: 0,
			textStyle: { color: '#a1a1aa', fontSize: 12 },
			itemWidth: 16,
			itemHeight: 2
		},
		grid: { left: 48, right: 16, top: 16, bottom: 40 },
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
				name: 'Active',
				type: 'line',
				data: data.map((d) => d.active),
				itemStyle: { color: STATUS_COLORS.ACTIVE },
				lineStyle: { width: 2 },
				smooth: true,
				symbol: 'circle',
				symbolSize: 5
			},
			{
				name: 'Completed',
				type: 'line',
				data: data.map((d) => d.completed),
				itemStyle: { color: STATUS_COLORS.COMPLETED },
				lineStyle: { width: 2 },
				smooth: true,
				symbol: 'circle',
				symbolSize: 5
			},
			{
				name: 'Expired',
				type: 'line',
				data: data.map((d) => d.expired),
				itemStyle: { color: STATUS_COLORS.EXPIRED },
				lineStyle: { width: 2 },
				smooth: true,
				symbol: 'circle',
				symbolSize: 5
			}
		]
	});
</script>

<ChartWrapper {options} />
