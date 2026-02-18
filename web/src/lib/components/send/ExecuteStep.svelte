<script lang="ts">
	import { sendStore } from '$lib/stores/send.svelte';
	import { getChainLabel, getExplorerTxUrl } from '$lib/utils/chains';
	import { truncateAddress, formatBalance } from '$lib/utils/formatting';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import type { TxResult } from '$lib/types';

	const store = sendStore;

	let step = $derived(store.state.step);
	let chain = $derived(store.state.chain);
	let preview = $derived(store.state.preview);
	let executeResult = $derived(store.state.executeResult);
	let txProgress = $derived(store.state.txProgress);
	let loading = $derived(store.state.loading);
	let error = $derived(store.state.error);

	let tokenLabel = $derived(
		preview?.token === 'NATIVE' && chain
			? CHAIN_NATIVE_SYMBOLS[chain]
			: preview?.token ?? ''
	);

	let isComplete = $derived(step === 'complete');

	// Show progress rows during execution, then final results.
	let displayRows = $derived<TxResult[]>(
		executeResult?.txResults ?? txProgress
	);

	function handleBack(): void {
		store.goBack();
	}

	async function handleExecute(): Promise<void> {
		await store.executeSweep();
	}

	function handleReset(): void {
		store.reset();
	}

	function getExplorerLink(txHash: string): string {
		if (!chain || !txHash) return '';
		// Use testnet for now — will be configurable.
		return getExplorerTxUrl(chain, txHash, 'testnet');
	}
</script>

{#if !isComplete}
	<!-- Execution Confirmation -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Step 4: Execute</div>
		</div>
		<div class="card-body">
			{#if preview && chain}
				<div class="confirm-message">
					This will send <strong>{formatBalance(preview.netAmount)} {tokenLabel}</strong>
					from <strong>{preview.fundedCount} addresses</strong>
					to <strong class="mono">{truncateAddress(preview.destination, 10)}</strong>
					on <strong>{getChainLabel(chain)}</strong>.
				</div>

				<div class="alert alert-info">
					<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
						<circle cx="9" cy="9" r="8" stroke="currentColor" stroke-width="1.5"/>
						<path d="M9 8v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<path d="M9 6.5v0" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
					</svg>
					This action is irreversible. Private keys will be derived, transactions signed and broadcast.
				</div>
			{/if}

			<!-- TX Progress during execution -->
			{#if txProgress.length > 0}
				<div class="table-wrapper">
					<table class="table">
						<thead>
							<tr>
								<th>#</th>
								<th>From</th>
								<th class="text-right">Amount</th>
								<th>TX Hash</th>
								<th>Status</th>
							</tr>
						</thead>
						<tbody>
							{#each txProgress as tx (tx.addressIndex)}
								<tr>
									<td class="text-muted">{tx.addressIndex}</td>
									<td><span class="mono">{truncateAddress(tx.fromAddress)}</span></td>
									<td class="mono text-right">{formatBalance(tx.amount)} {tokenLabel}</td>
									<td>
										{#if tx.txHash}
											<a href={getExplorerLink(tx.txHash)} target="_blank" rel="noopener" class="tx-link mono">
												{truncateAddress(tx.txHash)}
											</a>
										{:else}
											<span class="text-muted">-</span>
										{/if}
									</td>
									<td>
										{#if tx.status === 'success'}
											<span class="badge badge-success">Sent</span>
										{:else if tx.status === 'failed'}
											<span class="badge badge-error">Failed</span>
										{:else}
											<span class="badge badge-default">Pending</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}

			<div class="action-bar">
				<button class="btn btn-ghost" onclick={handleBack} disabled={loading}>
					<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<path d="M10 3L5 8l5 5"/>
					</svg>
					Back
				</button>
				<button
					class="btn btn-danger"
					onclick={handleExecute}
					disabled={loading}
				>
					{#if loading}
						<span class="spinner"></span>
						Executing...
					{:else}
						Execute Sweep
					{/if}
				</button>
			</div>
		</div>
	</div>
{:else}
	<!-- Completion Results -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Sweep Complete</div>
			{#if executeResult}
				<div class="result-badges">
					<span class="badge badge-success">{executeResult.successCount} succeeded</span>
					{#if executeResult.failCount > 0}
						<span class="badge badge-error">{executeResult.failCount} failed</span>
					{/if}
				</div>
			{/if}
		</div>
		<div class="card-body">
			{#if executeResult}
				<!-- Summary -->
				<div class="summary-grid">
					<span class="summary-label">Chain</span>
					<span class="summary-value">{chain}</span>

					<span class="summary-label">Token</span>
					<span class="summary-value">{tokenLabel}</span>

					<span class="summary-label">Total swept</span>
					<span class="summary-value summary-value-lg">{formatBalance(executeResult.totalSwept)} {tokenLabel}</span>

					<span class="summary-label">Transactions</span>
					<span class="summary-value">{executeResult.txResults.length}</span>
				</div>

				<!-- Results Table -->
				<div class="table-wrapper" style="margin-top:1.25rem;">
					<table class="table">
						<thead>
							<tr>
								<th>#</th>
								<th>From</th>
								<th class="text-right">Amount</th>
								<th>TX Hash</th>
								<th>Status</th>
							</tr>
						</thead>
						<tbody>
							{#each executeResult.txResults as tx (tx.addressIndex)}
								<tr>
									<td class="text-muted">{tx.addressIndex}</td>
									<td><span class="mono">{truncateAddress(tx.fromAddress)}</span></td>
									<td class="mono text-right">{formatBalance(tx.amount)} {tokenLabel}</td>
									<td>
										{#if tx.txHash}
											<a href={getExplorerLink(tx.txHash)} target="_blank" rel="noopener" class="tx-link mono">
												{truncateAddress(tx.txHash)}
											</a>
										{:else}
											<span class="text-muted">-</span>
										{/if}
									</td>
									<td>
										{#if tx.status === 'success'}
											<span class="badge badge-success">Sent</span>
										{:else}
											<span class="badge badge-error" title={tx.error ?? ''}>Failed</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}

			{#if error}
				<div class="alert alert-error">{error}</div>
			{/if}

			<div class="action-bar">
				<div></div>
				<button class="btn btn-primary" onclick={handleReset}>
					New Sweep
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
	}

	.mb-6 { margin-bottom: 1.5rem; }

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

	.card-body { padding: 1.25rem; }

	.confirm-message {
		font-size: 0.9375rem;
		color: var(--color-text-primary);
		line-height: 1.6;
		margin-bottom: 1.25rem;
	}

	.mono { font-family: var(--font-mono); font-size: 0.8125rem; }

	.summary-grid {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.5rem 1.5rem;
		align-items: baseline;
	}

	.summary-label { font-size: 0.8125rem; color: var(--color-text-muted); white-space: nowrap; }
	.summary-value { font-size: 0.8125rem; color: var(--color-text-primary); font-weight: 500; }
	.summary-value-lg { font-size: 1.125rem; font-weight: 600; }

	.result-badges { display: flex; gap: 0.5rem; }

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

	.badge-success { background: var(--color-success-muted); color: var(--color-success); }
	.badge-error { background: var(--color-error-muted); color: var(--color-error); }
	.badge-default { background: var(--color-accent-muted); color: var(--color-accent-text); }

	.table-wrapper { overflow-x: auto; }
	.table { width: 100%; border-collapse: collapse; }

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

	.table tr:last-child td { border-bottom: none; }
	.text-right { text-align: right; }
	.text-muted { color: var(--color-text-muted); }

	.tx-link {
		color: var(--color-accent-text);
		text-decoration: none;
	}

	.tx-link:hover { text-decoration: underline; }

	.alert {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		margin-top: 1rem;
	}

	.alert-info { background: var(--color-info-muted); color: var(--color-info); }
	.alert-error { background: var(--color-error-muted); color: var(--color-error); }

	.alert-icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}

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

	.btn:disabled { opacity: 0.4; cursor: not-allowed; }
	.btn-primary { background: var(--color-accent); color: white; }
	.btn-primary:hover:not(:disabled) { background: var(--color-accent-hover); }
	.btn-danger { background: var(--color-error-muted); color: var(--color-error); }
	.btn-danger:hover:not(:disabled) { background: var(--color-error); color: white; }

	.btn-ghost {
		background: transparent;
		color: var(--color-text-secondary);
	}

	.btn-ghost:hover:not(:disabled) {
		color: var(--color-text-primary);
		background: var(--color-bg-surface-hover);
	}

	.spinner {
		width: 14px;
		height: 14px;
		border: 2px solid transparent;
		border-top: 2px solid currentColor;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>
