<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Points</title>
</svelte:head>

<Header title="Points" subtitle="Points ledger and claim management" />

<div class="page-body">
	<p class="placeholder-text">Points management coming in a future phase.</p>
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
