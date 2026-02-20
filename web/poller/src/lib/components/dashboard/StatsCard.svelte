<script lang="ts">
	interface Props {
		label: string;
		value: string;
		hint?: string;
	}

	let { label, value, hint }: Props = $props();
</script>

<div class="card">
	<div class="stat">
		<div class="stat-label">{label}</div>
		<div class="stat-value">{value}</div>
		{#if hint}
			<div class="stat-hint">{hint}</div>
		{/if}
	</div>
</div>

<style>
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		padding: 1.25rem 1.5rem;
	}

	.stat {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.stat-label {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.stat-value {
		font-size: 1.5rem;
		font-weight: 600;
		font-family: var(--font-mono);
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
	}

	.stat-hint {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin-top: 0.125rem;
	}
</style>
