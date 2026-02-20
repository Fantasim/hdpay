<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Overview</title>
</svelte:head>

<Header title="Overview" subtitle="System health and activity at a glance" />

<div class="page-body">
	<p class="placeholder-text">Dashboard overview coming in a future phase.</p>
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
