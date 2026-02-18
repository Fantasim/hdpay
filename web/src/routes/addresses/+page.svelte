<script lang="ts">
	import Header from '$lib/components/layout/Header.svelte';
</script>

<Header title="Addresses" />

<div class="placeholder">
	<p class="text-muted">Address Explorer will be built in Phase 3.</p>
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
