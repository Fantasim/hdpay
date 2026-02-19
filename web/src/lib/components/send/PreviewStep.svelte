<script lang="ts">
	import { sendStore } from '$lib/stores/send.svelte';
	import { getChainLabel } from '$lib/utils/chains';
	import { truncateAddress, formatRawBalance } from '$lib/utils/formatting';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import type { Chain } from '$lib/types';

	const store = sendStore;

	let preview = $derived(store.state.preview);
	let chain = $derived(store.state.chain);
	let error = $derived(store.state.error);

	let tokenLabel = $derived(
		preview?.token === 'NATIVE' && chain
			? CHAIN_NATIVE_SYMBOLS[chain]
			: preview?.token ?? ''
	);

	let chainLabel = $derived(chain ? getChainLabel(chain) : '');

	function handleBack(): void {
		store.goBack();
	}

	function handleContinue(): void {
		store.advanceFromPreview();
	}
</script>

{#if preview && chain}
	<!-- Transaction Summary Card -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Transaction Summary</div>
		</div>
		<div class="card-body">
			<div class="summary-grid">
				<span class="summary-label">Chain</span>
				<span class="summary-value">
					<span class="badge badge-{chain.toLowerCase()}">{chain}</span>
					{chainLabel}
				</span>

				<span class="summary-label">Token</span>
				<span class="summary-value">{tokenLabel}</span>

				<span class="summary-label">Funded addresses</span>
				<span class="summary-value">{preview.fundedCount}</span>

				<span class="summary-label">Total amount</span>
				<span class="summary-value summary-value-lg">{formatRawBalance(preview.totalAmount, chain as Chain, preview.token)} {tokenLabel}</span>

				<span class="summary-label">Net amount</span>
				<span class="summary-value">{formatRawBalance(preview.netAmount, chain as Chain, preview.token)} {tokenLabel}</span>

				<span class="summary-label">Destination</span>
				<span class="summary-value summary-value-mono">{preview.destination}</span>
			</div>
		</div>
	</div>

	<!-- Fee Estimate Card -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Fee Estimate</div>
		</div>
		<div class="card-body">
			<div class="fee-grid">
				<span class="summary-label">Estimated fee</span>
				<span class="summary-value">
					<span class="font-mono">{formatRawBalance(preview.feeEstimate, chain as Chain, 'NATIVE')}</span>
					{chain ? CHAIN_NATIVE_SYMBOLS[chain] : ''}
				</span>

				<span class="summary-label">Transactions</span>
				<span class="summary-value">
					{preview.txCount}
					{#if preview.txCount > 1}
						<span class="tx-count-hint">(one per funded address, each costs gas)</span>
					{/if}
				</span>

				{#if preview.needsGasPreSeed}
					<span class="summary-label">Gas pre-seed needed</span>
					<span class="summary-value">
						<span class="badge badge-warning">Yes</span>
						<span class="text-muted">{preview.gasPreSeedCount} addresses need gas</span>
					</span>
				{/if}
			</div>

			{#if preview.needsGasPreSeed}
				<div class="alert alert-warning">
					<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
						<path d="M9 2L1.5 15h15L9 2z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
						<path d="M9 7v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<circle cx="9" cy="13" r="0.5" fill="currentColor"/>
					</svg>
					<span>{preview.gasPreSeedCount} addresses have tokens but no gas for transfer fees. Gas pre-seeding will be required before execution.</span>
				</div>
			{/if}
		</div>
	</div>

	<!-- Funded Addresses Table -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Funded Addresses</div>
			<span class="badge badge-default">{preview.fundedCount} addresses</span>
		</div>
		<div class="card-body">
			<div class="table-wrapper">
				<table class="table">
					<thead>
						<tr>
							<th>#</th>
							<th>Address</th>
							<th class="text-right">Balance</th>
							<th>Gas Status</th>
						</tr>
					</thead>
					<tbody>
						{#each preview.fundedAddresses as addr (addr.addressIndex)}
							<tr>
								<td class="text-muted">{addr.addressIndex.toLocaleString()}</td>
								<td>
									<span class="mono">{truncateAddress(addr.address)}</span>
								</td>
								<td class="mono text-right">{formatRawBalance(addr.balance, chain as Chain, preview.token)} {tokenLabel}</td>
								<td>
									{#if addr.hasGas}
										<span class="badge badge-success">Has Gas</span>
									{:else}
										<span class="badge badge-warning">Needs Gas</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	</div>

	<!-- Error -->
	{#if error}
		<div class="alert alert-error">
			{error}
		</div>
	{/if}

	<!-- Action Buttons -->
	<div class="action-bar">
		<button class="btn btn-ghost" onclick={handleBack}>
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
				<path d="M10 3L5 8l5 5"/>
			</svg>
			Back
		</button>
		<button class="btn btn-primary" onclick={handleContinue}>
			{preview.needsGasPreSeed ? 'Continue to Gas Pre-Seed' : 'Continue to Execute'}
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
				<path d="M6 3l5 5-5 5"/>
			</svg>
		</button>
	</div>
{/if}

<style>
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
	}

	.mb-6 {
		margin-bottom: 1.5rem;
	}

	.card-header {
		padding: 1rem 1.25rem;
		border-bottom: 1px solid var(--color-border-subtle);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.card-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.card-body {
		padding: 1.25rem;
	}

	.summary-grid,
	.fee-grid {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.5rem 1.5rem;
		align-items: baseline;
	}

	.summary-label {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	.summary-value {
		font-size: 0.8125rem;
		color: var(--color-text-primary);
		font-weight: 500;
	}

	.summary-value-lg {
		font-size: 1.125rem;
		font-weight: 600;
	}

	.summary-value-mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		word-break: break-all;
	}

	.tx-count-hint {
		color: var(--color-text-muted);
		font-weight: 400;
		font-size: 0.75rem;
		margin-left: 0.375rem;
	}

	.text-muted {
		color: var(--color-text-muted);
		margin-left: 0.5rem;
	}

	.font-mono {
		font-family: var(--font-mono);
	}

	/* Badges */
	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		font-size: 0.6875rem;
		font-weight: 600;
		letter-spacing: 0.02em;
		text-transform: uppercase;
	}

	.badge-btc { background: var(--color-btc-muted); color: var(--color-btc); }
	.badge-bsc { background: var(--color-bsc-muted); color: var(--color-bsc); }
	.badge-sol { background: var(--color-sol-muted); color: var(--color-sol); }
	.badge-success { background: var(--color-success-muted); color: var(--color-success); }
	.badge-warning { background: var(--color-warning-muted); color: var(--color-warning); }
	.badge-default { background: var(--color-accent-muted); color: var(--color-accent-text); }

	/* Table */
	.table-wrapper {
		overflow-x: auto;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
	}

	.table th {
		font-size: 0.6875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		padding: 0.625rem 0.75rem;
		border-bottom: 1px solid var(--color-border-subtle);
		text-align: left;
	}

	.table td {
		font-size: 0.8125rem;
		padding: 0.625rem 0.75rem;
		border-bottom: 1px solid var(--color-border-subtle);
		color: var(--color-text-primary);
	}

	.table tr:last-child td {
		border-bottom: none;
	}

	.text-right { text-align: right; }

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	/* Alerts */
	.alert {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		margin-top: 1rem;
	}

	.alert-warning {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.alert-error {
		background: var(--color-error-muted);
		color: var(--color-error);
		margin-bottom: 1rem;
	}

	.alert-icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}

	/* Action bar */
	.action-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 1.5rem;
		margin-top: 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
	}

	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 150ms ease;
		border: none;
		white-space: nowrap;
	}

	.btn-primary {
		background: var(--color-accent);
		color: white;
	}

	.btn-primary:hover { background: var(--color-accent-hover); }

	.btn-ghost {
		background: transparent;
		color: var(--color-text-secondary);
	}

	.btn-ghost:hover {
		color: var(--color-text-primary);
		background: var(--color-bg-surface-hover);
	}
</style>
