<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Errors</title>
</svelte:head>

<Header title="Errors" subtitle="Discrepancies, system errors, and stale transactions" />

<div class="page-body">
	<p class="placeholder-text">Error dashboard coming in a future phase.</p>
</div>

<style>
	.page-body {
		padding: 1.5rem 2rem;
	}

	.placeholder-text {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
