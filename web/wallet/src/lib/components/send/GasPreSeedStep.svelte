<script lang="ts">
	import { sendStore } from '$lib/stores/send.svelte';
	import { truncateAddress, formatRawBalance } from '$lib/utils/formatting';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import type { Chain } from '$lib/types';

	const store = sendStore;

	let preview = $derived(store.state.preview);
	let chain = $derived(store.state.chain);
	let gasResult = $derived(store.state.gasPreSeedResult);
	let loading = $derived(store.state.loading);
	let error = $derived(store.state.error);

	// Whether this is a SOL fee payer flow (no gas pre-seed API call needed).
	let isSOLFeePayer = $derived(chain === 'SOL');
	let nativeSymbol = $derived(chain ? CHAIN_NATIVE_SYMBOLS[chain] : 'gas');

	// Source index: default to 0 (first funded address).
	let sourceIndex = $state(0);

	// Skip warning modal state (BSC only).
	let showSkipWarning = $state(false);

	let addressesNeedingGas = $derived(
		preview?.fundedAddresses.filter((a) => !a.hasGas) ?? []
	);

	function handleBack(): void {
		store.goBack();
	}

	async function handleExecutePreSeed(): Promise<void> {
		await store.executeGasPreSeed(sourceIndex);
	}

	function handleConfirmFeePayer(): void {
		store.confirmFeePayerIndex(sourceIndex);
	}

	function handleSkipClick(): void {
		showSkipWarning = true;
	}

	function handleSkipConfirm(): void {
		showSkipWarning = false;
		store.skipGasPreSeed();
	}

	function handleSkipCancel(): void {
		showSkipWarning = false;
	}
</script>

<div class="card mb-6">
	<div class="card-header">
		<div class="card-title">Step 3: {isSOLFeePayer ? 'Fee Payer Selection' : 'Gas Pre-Seed'}</div>
		<span class="badge badge-warning">{addressesNeedingGas.length} addresses need {nativeSymbol}</span>
	</div>
	<div class="card-body">
		{#if isSOLFeePayer}
			<p class="description">
				These addresses hold tokens but have no SOL for transaction fees.
				Select a fee payer address — it will pay all transaction fees on their behalf.
			</p>
		{:else}
			<p class="description">
				These addresses hold tokens but have no native gas for transfer fees.
				Gas pre-seeding will send a small amount of {nativeSymbol} from a source address to each.
			</p>
		{/if}

		<!-- Source Index Input -->
		<div class="form-group">
			<label class="form-label" for="gas-source">{isSOLFeePayer ? 'Fee Payer Address Index' : 'Source Address Index'}</label>
			<input
				id="gas-source"
				type="number"
				class="form-input"
				bind:value={sourceIndex}
				min="0"
				placeholder="0"
			/>
			<div class="form-hint">Address index that holds {nativeSymbol} to {isSOLFeePayer ? 'pay transaction fees' : 'fund gas'}</div>
		</div>

		<!-- Addresses needing gas -->
		<div class="table-wrapper">
			<table class="table">
				<thead>
					<tr>
						<th>#</th>
						<th>Address</th>
						<th class="text-right">Token Balance</th>
					</tr>
				</thead>
				<tbody>
					{#each addressesNeedingGas as addr (addr.addressIndex)}
						<tr>
							<td class="text-muted">{addr.addressIndex.toLocaleString()}</td>
							<td><span class="mono">{truncateAddress(addr.address)}</span></td>
							<td class="mono text-right">{formatRawBalance(addr.balance, chain as Chain, preview?.token ?? 'NATIVE')}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Gas Pre-Seed Results -->
		{#if gasResult}
			<div class="result-section">
				<div class="result-header">Gas Pre-Seed Results</div>
				<div class="result-summary">
					<span class="badge badge-success">{gasResult.successCount} succeeded</span>
					{#if gasResult.failCount > 0}
						<span class="badge badge-error">{gasResult.failCount} failed</span>
					{/if}
					<span class="text-muted">Total sent: {formatRawBalance(gasResult.totalSent, chain as Chain, 'NATIVE')} {nativeSymbol}</span>
				</div>
				<div class="table-wrapper">
					<table class="table">
						<thead>
							<tr>
								<th>#</th>
								<th>Address</th>
								<th>TX Hash</th>
								<th>Status</th>
							</tr>
						</thead>
						<tbody>
							{#each gasResult.txResults as tx (tx.addressIndex)}
								<tr>
									<td class="text-muted">{tx.addressIndex}</td>
									<td><span class="mono">{truncateAddress(tx.fromAddress)}</span></td>
									<td><span class="mono">{truncateAddress(tx.txHash)}</span></td>
									<td>
										{#if tx.status === 'success'}
											<span class="badge badge-success">Sent</span>
										{:else}
											<span class="badge badge-error">Failed</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}

		<!-- Error -->
		{#if error}
			<div class="alert alert-error">{error}</div>
		{/if}

		<!-- Actions -->
		<div class="action-bar">
			<button class="btn btn-ghost" onclick={handleBack}>
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
					<path d="M10 3L5 8l5 5"/>
				</svg>
				Back
			</button>
			<div class="action-right">
				{#if isSOLFeePayer}
					<!-- SOL: fee payer just needs to be selected, then continue to execute -->
					<button class="btn btn-primary" onclick={handleConfirmFeePayer}>
						Continue to Execute
						<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
							<path d="M6 3l5 5-5 5"/>
						</svg>
					</button>
				{:else}
					<!-- BSC: gas pre-seed requires sending BNB to each address -->
					<button class="btn btn-ghost" onclick={handleSkipClick}>
						Skip
					</button>
					{#if gasResult}
						<button class="btn btn-primary" onclick={() => store.goToStep('execute')}>
							Continue to Execute
							<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
								<path d="M6 3l5 5-5 5"/>
							</svg>
						</button>
					{:else}
						<button
							class="btn btn-primary"
							onclick={handleExecutePreSeed}
							disabled={loading}
						>
							{loading ? 'Sending Gas...' : 'Execute Gas Pre-Seed'}
						</button>
					{/if}
				{/if}
			</div>
		</div>
	</div>
</div>

<!-- Skip Warning Modal -->
{#if showSkipWarning}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="modal-overlay" onclick={handleSkipCancel}>
		<div class="modal" onclick={(e) => e.stopPropagation()}>
			<div class="modal-header">
				<h3 class="modal-title">Skip Gas Pre-Seed?</h3>
			</div>
			<div class="modal-body">
				<p class="modal-warning-text">
					<strong>{addressesNeedingGas.length} addresses</strong> have tokens but no native gas ({nativeSymbol}) for transfer fees.
				</p>
				<p class="modal-warning-text">
					If you skip gas pre-seeding, these addresses <strong>will fail</strong> during the token sweep because they cannot pay transaction fees.
				</p>
			</div>
			<div class="modal-footer">
				<button class="btn btn-primary" onclick={handleSkipCancel}>
					Go Back
				</button>
				<button class="btn btn-ghost" onclick={handleSkipConfirm}>
					Skip Anyway
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

	.description {
		font-size: 0.8125rem;
		color: var(--color-text-secondary);
		margin-bottom: 1.25rem;
		line-height: 1.5;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
		margin-bottom: 1.25rem;
	}

	.form-label {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.form-input {
		padding: 0.5rem 0.75rem;
		background: var(--color-bg-input);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		color: var(--color-text-primary);
		font-size: 0.8125rem;
		font-family: var(--font-mono);
		outline: none;
		transition: border-color 150ms ease;
		max-width: 200px;
	}

	.form-input:focus { border-color: var(--color-border-focus); }
	.form-hint { font-size: 0.6875rem; color: var(--color-text-muted); }

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

	.badge-warning { background: var(--color-warning-muted); color: var(--color-warning); }
	.badge-success { background: var(--color-success-muted); color: var(--color-success); }
	.badge-error { background: var(--color-error-muted); color: var(--color-error); }

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

	.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	.result-section {
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border-subtle);
	}

	.result-header {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin-bottom: 0.75rem;
	}

	.result-summary {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 1rem;
	}

	.alert {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		margin-top: 1rem;
	}

	.alert-error { background: var(--color-error-muted); color: var(--color-error); }

	.action-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 1.5rem;
		margin-top: 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
	}

	.action-right {
		display: flex;
		gap: 0.5rem;
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

	.btn-ghost {
		background: transparent;
		color: var(--color-text-secondary);
	}

	.btn-ghost:hover {
		color: var(--color-text-primary);
		background: var(--color-bg-surface-hover);
	}

	/* Modal */
	.modal-overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
		backdrop-filter: blur(2px);
	}

	.modal {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 12px;
		width: 100%;
		max-width: 440px;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
	}

	.modal-header {
		padding: 1.25rem 1.5rem;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.modal-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.modal-body { padding: 1.5rem; }

	.modal-warning-text {
		font-size: 0.875rem;
		color: var(--color-text-secondary);
		line-height: 1.6;
		margin: 0 0 0.75rem 0;
	}

	.modal-warning-text:last-child { margin-bottom: 0; }

	.modal-footer {
		padding: 1rem 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
	}
</style>
