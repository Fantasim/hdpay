<script lang="ts">
	import type { PortfolioResponse } from '$lib/types';
	import { formatUsd, formatNumber, formatRelativeTime } from '$lib/utils/formatting';
	import { SUPPORTED_CHAINS } from '$lib/constants';

	interface Props {
		portfolio: PortfolioResponse | null;
		loading: boolean;
	}

	let { portfolio, loading }: Props = $props();

	let totalAddresses = $derived(
		portfolio ? portfolio.chains.reduce((sum, c) => sum + c.addressCount, 0) : 0
	);

	let totalFunded = $derived(
		portfolio ? portfolio.chains.reduce((sum, c) => sum + c.fundedCount, 0) : 0
	);

	let chainCount = $derived(SUPPORTED_CHAINS.length);
</script>

<!-- Portfolio Total -->
<div class="portfolio-total">
	<div class="portfolio-label">Total Portfolio Value</div>
	{#if loading && !portfolio}
		<div class="stat-value stat-value-lg font-mono skeleton">Loading...</div>
	{:else}
		<div class="stat-value stat-value-lg font-mono">{formatUsd(portfolio?.totalUsd ?? 0)}</div>
	{/if}
</div>

<!-- Quick Stats -->
<div class="stats-grid">
	<div class="card">
		<div class="stat">
			<span class="stat-label">Total Addresses</span>
			<span class="stat-value">{formatNumber(totalAddresses)}</span>
		</div>
	</div>
	<div class="card">
		<div class="stat">
			<span class="stat-label">Funded</span>
			<span class="stat-value text-success">{formatNumber(totalFunded)}</span>
		</div>
	</div>
	<div class="card">
		<div class="stat">
			<span class="stat-label">Chains</span>
			<span class="stat-value">{chainCount}</span>
		</div>
	</div>
	<div class="card">
		<div class="stat">
			<span class="stat-label">Last Scan</span>
			<span class="stat-value stat-value-sm">{formatRelativeTime(portfolio?.lastScan ?? null)}</span>
		</div>
	</div>
</div>

<style>
	.portfolio-total {
		margin-bottom: 0.5rem;
	}

	.portfolio-label {
		font-size: 0.875rem;
		color: var(--color-text-muted);
		margin-bottom: 0.5rem;
	}

	.stat-value-lg {
		font-size: 2rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.02em;
		margin-bottom: 1.5rem;
	}

	.stat-value-sm {
		font-size: 1.125rem !important;
	}

	.skeleton {
		color: var(--color-text-muted);
		animation: pulse 1.5s ease-in-out infinite;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	.stats-grid {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 1rem;
		margin-bottom: 2rem;
	}

	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border);
		border-radius: 8px;
		padding: 1rem 1.25rem;
	}

	.stat {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.stat-label {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.stat-value {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.text-success {
		color: var(--color-success);
	}
</style>
