<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		actions?: Snippet;
	}

	let { title, actions }: Props = $props();
</script>

<header class="page-header">
	<h1 class="page-title">{title}</h1>
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
		margin-bottom: 1.5rem;
		height: var(--header-height);
	}

	.page-title {
		font-size: 1.375rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
		margin: 0;
	}

	.page-actions {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
</style>
