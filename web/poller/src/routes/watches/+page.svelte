<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Watches</title>
</svelte:head>

<Header title="Watches" subtitle="Active and past address watch sessions" />

<div class="page-body">
	<p class="placeholder-text">Watch management coming in a future phase.</p>
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
