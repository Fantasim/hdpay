<script lang="ts">
	import { TIME_RANGES, TIME_RANGE_LABELS } from '$lib/constants';
	import type { TimeRange } from '$lib/types';

	interface Props {
		selected: TimeRange;
		onchange: (range: TimeRange) => void;
	}

	let { selected, onchange }: Props = $props();
</script>

<div class="time-range">
	{#each TIME_RANGES as range}
		<button
			class="time-range-btn"
			class:active={selected === range}
			type="button"
			onclick={() => onchange(range)}
		>
			{TIME_RANGE_LABELS[range]}
		</button>
	{/each}
</div>

<style>
	.time-range {
		display: flex;
		gap: 0.25rem;
	}

	.time-range-btn {
		padding: 0.375rem 0.75rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-md);
		background: transparent;
		color: var(--color-text-muted);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
		white-space: nowrap;
	}

	.time-range-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-border-focus);
	}

	.time-range-btn.active {
		background: var(--color-accent-muted);
		color: var(--color-accent-text);
		border-color: var(--color-accent-muted);
	}
</style>
