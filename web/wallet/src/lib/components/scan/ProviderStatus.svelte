<script lang="ts">
	import { onMount } from 'svelte';
	import { getProviderHealth } from '$lib/utils/api';
	import { SUPPORTED_CHAINS } from '$lib/constants';
	import type { ProviderHealthMap, ProviderHealth, Chain } from '$lib/types';
	import { statusDotClass, statusLabel } from '$lib/utils/providers';

	let providers: ProviderHealthMap | null = $state(null);
	let loading = $state(true);
	let error: string | null = $state(null);

	async function fetchHealth(): Promise<void> {
		try {
			const res = await getProviderHealth();
			providers = res.data;
			error = null;
		} catch (e) {
			error = 'Failed to load provider health';
		} finally {
			loading = false;
		}
	}

	function allProviders(chain: Chain): ProviderHealth[] {
		if (!providers) return [];
		return providers[chain] ?? [];
	}

	onMount(() => {
		fetchHealth();
	});
</script>

<div class="card">
	<div class="card-header">
		<div class="card-title">Provider Status</div>
	</div>
	<div class="card-body">
		{#if loading}
			<div class="loading">Loading provider status...</div>
		{:else if error}
			<div class="error-text">{error}</div>
		{:else if providers}
			<div class="provider-grid">
				{#each SUPPORTED_CHAINS as chain (chain)}
					{#each allProviders(chain) as provider (provider.name)}
						<div class="provider-item" title={provider.lastErrorMsg || ''}>
							<span class="provider-dot {statusDotClass(provider)}"></span>
							<div>
								<div class="provider-name">{provider.name}</div>
								<div class="provider-meta">
									<span class="provider-chain">{provider.chain}</span>
									<span class="provider-status">{statusLabel(provider)}</span>
								</div>
							</div>
						</div>
					{/each}
				{/each}
			</div>
			{#if SUPPORTED_CHAINS.every(c => allProviders(c).length === 0)}
				<div class="empty-text">No providers configured yet. Run a scan to populate.</div>
			{/if}
		{/if}
	</div>
</div>

<style>
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
	}

	.card-header {
		padding: 1rem 1.25rem;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.card-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.card-body {
		padding: 1.25rem;
	}

	.provider-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 1rem;
	}

	.provider-item {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.provider-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.provider-dot-healthy {
		background: var(--color-success);
	}

	.provider-dot-degraded {
		background: var(--color-warning);
	}

	.provider-dot-down {
		background: var(--color-error);
	}

	.provider-name {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.provider-meta {
		display: flex;
		gap: 0.375rem;
		align-items: center;
	}

	.provider-chain {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		background: var(--color-bg-elevated);
		padding: 0 0.25rem;
		border-radius: 3px;
	}

	.provider-status {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}

	.loading, .error-text, .empty-text {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		text-align: center;
		padding: 0.5rem 0;
	}

	.error-text {
		color: var(--color-error);
	}
</style>
