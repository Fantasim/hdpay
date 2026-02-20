<script lang="ts">
	import { Chart } from 'svelte-echarts';
	import { init, use } from 'echarts/core';
	import { BarChart, LineChart, PieChart } from 'echarts/charts';
	import {
		GridComponent,
		TooltipComponent,
		LegendComponent,
		TitleComponent
	} from 'echarts/components';
	import { CanvasRenderer } from 'echarts/renderers';
	import type { EChartsOption } from 'echarts';

	// Register only what we need — tree-shaken bundle
	use([
		BarChart,
		LineChart,
		PieChart,
		GridComponent,
		TooltipComponent,
		LegendComponent,
		TitleComponent,
		CanvasRenderer
	]);

	interface Props {
		options: EChartsOption;
		height?: string;
	}

	let { options, height = '20rem' }: Props = $props();
</script>

<div class="chart-container" style="height: {height};">
	<Chart {init} {options} theme="dark" />
</div>

<style>
	.chart-container {
		width: 100%;
	}

	.chart-container :global(div) {
		background: transparent !important;
	}
</style>
