<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import ScanControl from '$lib/components/scan/ScanControl.svelte';
	import ScanProgress from '$lib/components/scan/ScanProgress.svelte';
	import ProviderStatus from '$lib/components/scan/ProviderStatus.svelte';
	import { scanStore } from '$lib/stores/scan.svelte';
	import { SUPPORTED_CHAINS } from '$lib/constants';
	import type { Chain } from '$lib/types';

	const store = scanStore;

	let sseStatus = $derived(store.state.sseStatus);

	onMount(() => {
		store.fetchStatus();
		store.connectSSE();

		return () => {
			store.disconnectSSE();
		};
	});
</script>

<Header title="Scan">
	{#snippet actions()}
		<span class="sse-indicator" class:connected={sseStatus === 'connected'} class:error={sseStatus === 'error'}>
			<span class="sse-dot"></span>
			{#if sseStatus === 'connected'}
				Live
			{:else if sseStatus === 'connecting'}
				Connecting...
			{:else if sseStatus === 'error'}
				Reconnecting...
			{:else}
				Disconnected
			{/if}
		</span>
	{/snippet}
</Header>

<!-- Error Banner -->
{#if store.state.error}
	<div class="error-banner">
		{store.state.error}
	</div>
{/if}

<!-- Scan Control Panel -->
<ScanControl />

<!-- Active & Recent Scans -->
<div class="section">
	<div class="section-title">Active &amp; Recent Scans</div>
	<div class="scan-list">
		{#each SUPPORTED_CHAINS as chain (chain)}
			<ScanProgress
				{chain}
				status={store.state.statuses[chain] ?? null}
				progress={store.state.progress[chain] ?? null}
				lastComplete={store.state.lastComplete[chain] ?? null}
				lastError={store.state.lastError[chain] ?? null}
			/>
		{/each}
	</div>
</div>

<!-- Provider Status -->
<ProviderStatus />

<style>
	.sse-indicator {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
	}

	.sse-indicator.connected {
		color: var(--color-success);
	}

	.sse-indicator.error {
		color: var(--color-warning);
	}

	.sse-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--color-text-muted);
	}

	.sse-indicator.connected .sse-dot {
		background: var(--color-success);
	}

	.sse-indicator.error .sse-dot {
		background: var(--color-warning);
	}

	.error-banner {
		margin-bottom: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		background: var(--color-error-muted);
		color: var(--color-error);
		font-size: 0.8125rem;
	}

	.section {
		margin-bottom: 2rem;
	}

	.section-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
		margin-bottom: 1rem;
	}

	.scan-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
</style>
