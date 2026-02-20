<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		subtitle?: string;
		actions?: Snippet;
	}

	let { title, subtitle, actions }: Props = $props();
</script>

<header class="page-header">
	<div>
		<h1 class="page-title">{title}</h1>
		{#if subtitle}
			<p class="page-subtitle">{subtitle}</p>
		{/if}
	</div>
	{#if actions}
		<div class="page-actions">
			{@render actions()}
		</div>
	{/if}
</header>

<style>
	.page-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1.25rem 2rem;
		border-bottom: 1px solid var(--color-border-subtle);
		min-height: var(--header-height);
	}

	.page-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
		margin: 0;
	}

	.page-subtitle {
		font-size: 0.8125rem;
		color: var(--color-text-secondary);
		margin-top: 0.25rem;
		margin-bottom: 0;
	}

	.page-actions {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
</style>
