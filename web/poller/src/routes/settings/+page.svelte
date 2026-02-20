<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<svelte:head>
	<title>Poller — Settings</title>
</svelte:head>

<Header title="Settings" subtitle="Tiers, watch defaults, and IP allowlist" />

<div class="page-body">
	<p class="placeholder-text">Settings panel coming in a future phase.</p>
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
