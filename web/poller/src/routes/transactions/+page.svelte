<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Transactions</title>
</svelte:head>

<Header title="Transactions" subtitle="All detected deposits across chains" />

<div class="page-body">
	<p class="placeholder-text">Transaction list coming in a future phase.</p>
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
