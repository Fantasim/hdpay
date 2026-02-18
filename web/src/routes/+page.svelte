<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<Header title="Dashboard" />

<div class="placeholder">
	<p class="text-muted">Dashboard will be built in Phase 6.</p>
</div>

<style>
	.placeholder {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 300px;
		border: 1px dashed var(--color-border);
		border-radius: 8px;
	}

	.text-muted {
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}
</style>
