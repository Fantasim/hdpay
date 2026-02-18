<script lang="ts">
	const providers = [
		{ name: 'Blockstream', chain: 'BTC', status: 'Healthy' },
		{ name: 'Mempool.space', chain: 'BTC', status: 'Healthy' },
		{ name: 'BscScan', chain: 'BSC', status: 'Healthy' },
		{ name: 'Solana RPC', chain: 'SOL', status: 'Healthy' }
	] as const;
</script>

<div class="card">
	<div class="card-header">
		<div class="card-title">Provider Status</div>
	</div>
	<div class="card-body">
		<div class="provider-grid">
			{#each providers as provider}
				<div class="provider-item">
					<span class="provider-dot provider-dot-healthy"></span>
					<div>
						<div class="provider-name">{provider.name}</div>
						<div class="provider-status">{provider.status}</div>
					</div>
				</div>
			{/each}
		</div>
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
		grid-template-columns: repeat(4, 1fr);
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

	.provider-name {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.provider-status {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}
</style>
