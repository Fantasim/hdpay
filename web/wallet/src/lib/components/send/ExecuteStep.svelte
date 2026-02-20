<script lang="ts">
	import { onMount } from 'svelte';
	import { sendStore } from '$lib/stores/send.svelte';
	import { getChainLabel, getExplorerTxUrl } from '$lib/utils/chains';
	import { truncateAddress, formatRawBalance, computeUsdValue as computeUsd } from '$lib/utils/formatting';
	import { getSettings, getPrices } from '$lib/utils/api';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import type { Chain, TxResult, PriceData } from '$lib/types';

	const store = sendStore;

	let step = $derived(store.state.step);
	let chain = $derived(store.state.chain);
	let preview = $derived(store.state.preview);
	let executeResult = $derived(store.state.executeResult);
	let txProgress = $derived(store.state.txProgress);
	let loading = $derived(store.state.loading);
	let error = $derived(store.state.error);
	let sseStatus = $derived(store.state.sseStatus);

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

	// Network mode from server config (fetched once on mount).
	let network: string = $state('mainnet');

	// USD prices for value display.
	let prices: PriceData | null = $state(null);

	// Confirmation modal state.
	let showConfirmModal = $state(false);
	let confirmInput = $state('');
	let countdown = $state(0);
	let countdownTimer: ReturnType<typeof setInterval> | null = $state(null);

	// Double-click guard (synchronous, not dependent on state update).
	let executeGuard = false;

	const CONFIRM_COUNTDOWN_SECONDS = 3;
	const CONFIRM_KEYWORD = 'CONFIRM';

	onMount(async () => {
		try {
			const [settingsRes, pricesRes] = await Promise.all([
				getSettings(),
				getPrices()
			]);
			if (settingsRes.data?.network) {
				network = settingsRes.data.network;
			}
			if (pricesRes.data?.prices) {
				prices = pricesRes.data.prices;
			}
		} catch {
			// Fall back to defaults if fetches fail.
		}
	});

	// Compute USD value from raw balance string using the extracted utility.
	function computeUsdValue(rawAmount: string): string | null {
		if (!prices || !chain || !preview) return null;
		return computeUsd(rawAmount, chain, preview.token, prices as unknown as Record<string, number>);
	}

	let netAmountUsd = $derived(preview ? computeUsdValue(preview.netAmount) : null);
	let feeUsd = $derived(preview ? computeUsdValue(preview.feeEstimate) : null);
	let totalSweptUsd = $derived(executeResult ? computeUsdValue(executeResult.totalSwept) : null);

	function handleBack(): void {
		store.goBack();
	}

	function openConfirmModal(): void {
		confirmInput = '';
		countdown = CONFIRM_COUNTDOWN_SECONDS;
		showConfirmModal = true;

		// Start countdown.
		countdownTimer = setInterval(() => {
			countdown--;
			if (countdown <= 0) {
				if (countdownTimer) clearInterval(countdownTimer);
				countdownTimer = null;
			}
		}, 1000);
	}

	function closeConfirmModal(): void {
		showConfirmModal = false;
		confirmInput = '';
		countdown = 0;
		if (countdownTimer) {
			clearInterval(countdownTimer);
			countdownTimer = null;
		}
	}

	let canConfirm = $derived(
		countdown <= 0 && confirmInput.trim().toUpperCase() === CONFIRM_KEYWORD && !loading
	);

	async function handleConfirmedExecute(): Promise<void> {
		if (!canConfirm || executeGuard) return;

		executeGuard = true;
		closeConfirmModal();

		try {
			await store.executeSweep();
		} finally {
			executeGuard = false;
		}
	}

	function handleReset(): void {
		store.reset();
	}

	function getExplorerLink(txHash: string): string {
		if (!chain || !txHash) return '';
		return getExplorerTxUrl(chain, txHash, network);
	}

	function copyToClipboard(text: string): void {
		navigator.clipboard.writeText(text);
	}
</script>

{#if !isComplete}
	<!-- Execution Confirmation -->
	<div class="card mb-6">
		<div class="card-header">
			<div class="card-title">Step 4: Execute</div>
			<!-- Network Badge -->
			{#if network === 'testnet'}
				<span class="badge badge-testnet">TESTNET</span>
			{:else}
				<span class="badge badge-mainnet">MAINNET</span>
			{/if}
		</div>
		<div class="card-body">
			{#if preview && chain}
				<!-- Sweep Summary -->
				<div class="summary-grid">
					<span class="summary-label">Chain</span>
					<span class="summary-value">{getChainLabel(chain)}</span>

					<span class="summary-label">Token</span>
					<span class="summary-value">{tokenLabel}</span>

					<span class="summary-label">Funded addresses</span>
					<span class="summary-value">{preview.fundedCount}</span>

					{#if preview.txCount > 1}
						<span class="summary-label">Transactions</span>
						<span class="summary-value">{preview.txCount} separate transactions</span>
					{/if}

					<span class="summary-label">Net amount</span>
					<span class="summary-value summary-value-lg">
						{formatRawBalance(preview.netAmount, chain as Chain, preview.token)} {tokenLabel}
						{#if netAmountUsd}
							<span class="usd-value">({netAmountUsd})</span>
						{/if}
					</span>

					<span class="summary-label">Estimated fee</span>
					<span class="summary-value">
						{formatRawBalance(preview.feeEstimate, chain as Chain, 'NATIVE')} {CHAIN_NATIVE_SYMBOLS[chain]}
						{#if feeUsd}
							<span class="usd-value">({feeUsd})</span>
						{/if}
					</span>

					<span class="summary-label">Destination</span>
					<span class="summary-value">
						<span class="destination-address mono">{preview.destination}</span>
						<button class="btn-copy" onclick={() => copyToClipboard(preview!.destination)} title="Copy address">
							<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
								<rect x="5" y="5" width="9" height="9" rx="1.5"/>
								<path d="M5 11H3.5A1.5 1.5 0 012 9.5v-7A1.5 1.5 0 013.5 1h7A1.5 1.5 0 0112 2.5V5"/>
							</svg>
						</button>
					</span>
				</div>

				<div class="alert alert-danger">
					<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
						<path d="M9 2L1.5 15h15L9 2z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
						<path d="M9 7v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<circle cx="9" cy="13" r="0.5" fill="currentColor"/>
					</svg>
					This action is irreversible. Private keys will be derived, transactions signed and broadcast. Verify the destination address carefully.
				</div>
			{/if}

			<!-- SSE Connection Warning -->
			{#if sseStatus === 'error'}
				<div class="alert alert-warning">
					<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
						<path d="M9 2L1.5 15h15L9 2z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
						<path d="M9 7v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
						<circle cx="9" cy="13" r="0.5" fill="currentColor"/>
					</svg>
					Real-time status updates disconnected. Check the Transactions page for latest status.
				</div>
			{/if}

			<!-- Progress counter during execution -->
			{#if loading && txProgress.length > 0}
				<div class="progress-info">
					{txProgress.length} of {preview?.fundedCount ?? '?'} transactions processed
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
									<td class="mono text-right">{formatRawBalance(tx.amount, chain as Chain, preview?.token ?? 'NATIVE')} {tokenLabel}</td>
									<td>
										{#if tx.txHash}
											<span class="tx-hash-cell">
												<a href={getExplorerLink(tx.txHash)} target="_blank" rel="noopener" class="tx-link mono">
													{truncateAddress(tx.txHash)}
												</a>
												<button class="btn-copy-sm" onclick={() => copyToClipboard(tx.txHash)} title="Copy TX hash">
													<svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
														<rect x="5" y="5" width="9" height="9" rx="1.5"/>
														<path d="M5 11H3.5A1.5 1.5 0 012 9.5v-7A1.5 1.5 0 013.5 1h7A1.5 1.5 0 0112 2.5V5"/>
													</svg>
												</button>
											</span>
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
				<div class="action-bar-right">
					{#if loading}
						<a class="btn btn-ghost btn-sm" href="/transactions">
							Check Transactions page
						</a>
					{/if}
					<button
						class="btn btn-danger"
						onclick={openConfirmModal}
						disabled={loading}
						style={loading ? 'pointer-events: none;' : ''}
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
	</div>

	<!-- Confirmation Modal -->
	{#if showConfirmModal}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-overlay" onclick={closeConfirmModal}>
			<div class="modal" onclick={(e) => e.stopPropagation()}>
				<div class="modal-header">
					<h3 class="modal-title">Confirm Sweep Execution</h3>
					{#if network === 'testnet'}
						<span class="badge badge-testnet">TESTNET</span>
					{:else}
						<span class="badge badge-mainnet">MAINNET</span>
					{/if}
				</div>
				<div class="modal-body">
					{#if preview && chain}
						<div class="modal-summary">
							<div class="modal-summary-row">
								<span class="modal-label">Amount</span>
								<span class="modal-value">
									{formatRawBalance(preview.netAmount, chain as Chain, preview.token)} {tokenLabel}
									{#if netAmountUsd}
										<span class="usd-value">({netAmountUsd})</span>
									{/if}
								</span>
							</div>
							<div class="modal-summary-row">
								<span class="modal-label">Fee</span>
								<span class="modal-value">
									{formatRawBalance(preview.feeEstimate, chain as Chain, 'NATIVE')} {CHAIN_NATIVE_SYMBOLS[chain]}
								</span>
							</div>
							<div class="modal-summary-row">
								<span class="modal-label">From</span>
								<span class="modal-value">{preview.fundedCount} addresses on {getChainLabel(chain)}</span>
							</div>
							<div class="modal-summary-row modal-summary-row-dest">
								<span class="modal-label">To</span>
								<span class="modal-value mono destination-address-modal">{preview.destination}</span>
							</div>
						</div>

						<div class="modal-confirm-section">
							<label class="modal-confirm-label" for="confirm-input">
								Type <strong>{CONFIRM_KEYWORD}</strong> to proceed
							</label>
							<input
								id="confirm-input"
								type="text"
								class="modal-confirm-input"
								bind:value={confirmInput}
								placeholder={CONFIRM_KEYWORD}
								autocomplete="off"
								spellcheck="false"
							/>
						</div>
					{/if}
				</div>
				<div class="modal-footer">
					<button class="btn btn-ghost" onclick={closeConfirmModal}>
						Cancel
					</button>
					<button
						class="btn btn-danger"
						onclick={handleConfirmedExecute}
						disabled={!canConfirm}
						style={!canConfirm ? 'pointer-events: none;' : ''}
					>
						{#if countdown > 0}
							Wait {countdown}s...
						{:else if confirmInput.trim().toUpperCase() !== CONFIRM_KEYWORD}
							Type CONFIRM
						{:else}
							Execute Sweep
						{/if}
					</button>
				</div>
			</div>
		</div>
	{/if}
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

					<span class="summary-label">Destination</span>
					<span class="summary-value mono" style="word-break:break-all;">{preview?.destination ?? ''}</span>

					<span class="summary-label">Total swept</span>
					<span class="summary-value summary-value-lg">
						{formatRawBalance(executeResult.totalSwept, chain as Chain, preview?.token ?? 'NATIVE')} {tokenLabel}
						{#if totalSweptUsd}
							<span class="usd-value">({totalSweptUsd})</span>
						{/if}
					</span>

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
									<td class="mono text-right">{formatRawBalance(tx.amount, chain as Chain, preview?.token ?? 'NATIVE')} {tokenLabel}</td>
									<td>
										{#if tx.txHash}
											<span class="tx-hash-cell">
												<a href={getExplorerLink(tx.txHash)} target="_blank" rel="noopener" class="tx-link mono">
													{truncateAddress(tx.txHash)}
												</a>
												<button class="btn-copy-sm" onclick={() => copyToClipboard(tx.txHash)} title="Copy TX hash">
													<svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
														<rect x="5" y="5" width="9" height="9" rx="1.5"/>
														<path d="M5 11H3.5A1.5 1.5 0 012 9.5v-7A1.5 1.5 0 013.5 1h7A1.5 1.5 0 0112 2.5V5"/>
													</svg>
												</button>
											</span>
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
				<a class="btn btn-ghost" href="/transactions">
					View Transactions
				</a>
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

	.mono { font-family: var(--font-mono); font-size: 0.8125rem; }

	.summary-grid {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.5rem 1.5rem;
		align-items: baseline;
		margin-bottom: 1.25rem;
	}

	.summary-label { font-size: 0.8125rem; color: var(--color-text-muted); white-space: nowrap; }
	.summary-value { font-size: 0.8125rem; color: var(--color-text-primary); font-weight: 500; }
	.summary-value-lg { font-size: 1.125rem; font-weight: 600; }

	.usd-value {
		color: var(--color-text-muted);
		font-size: 0.8125rem;
		font-weight: 400;
		margin-left: 0.375rem;
	}

	.destination-address {
		word-break: break-all;
		line-height: 1.4;
	}

	.btn-copy {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.25rem;
		border: none;
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		border-radius: 3px;
		margin-left: 0.375rem;
		vertical-align: middle;
	}

	.btn-copy:hover { color: var(--color-text-primary); background: var(--color-bg-surface-hover); }

	.tx-hash-cell { display: inline-flex; align-items: center; gap: 0.25rem; }

	.btn-copy-sm {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.125rem;
		border: none;
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		border-radius: 3px;
	}

	.btn-copy-sm:hover { color: var(--color-text-primary); }

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
	.badge-mainnet { background: #fef2f2; color: #dc2626; font-weight: 700; }
	.badge-testnet { background: #fefce8; color: #ca8a04; font-weight: 700; }

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

	.alert-danger { background: var(--color-error-muted); color: var(--color-error); }
	.alert-warning { background: var(--color-warning-muted, #fefce8); color: var(--color-warning, #ca8a04); }
	.alert-error { background: var(--color-error-muted); color: var(--color-error); }

	.alert-icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}

	.progress-info {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		padding: 0.5rem 0;
		font-weight: 500;
	}

	.action-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 1.5rem;
		margin-top: 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
	}

	.action-bar-right {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.btn-sm {
		padding: 0.375rem 0.75rem;
		font-size: 0.75rem;
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
		text-decoration: none;
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

	/* Confirmation Modal */
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
		max-width: 520px;
		box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
	}

	.modal-header {
		padding: 1.25rem 1.5rem;
		border-bottom: 1px solid var(--color-border-subtle);
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.modal-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-summary {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 1.5rem;
	}

	.modal-summary-row {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}

	.modal-summary-row-dest {
		flex-direction: column;
		gap: 0.25rem;
	}

	.modal-label {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		font-weight: 500;
	}

	.modal-value {
		font-size: 0.8125rem;
		color: var(--color-text-primary);
		font-weight: 600;
	}

	.destination-address-modal {
		word-break: break-all;
		line-height: 1.4;
		padding: 0.5rem 0.75rem;
		background: var(--color-bg-input, var(--color-bg-surface-hover));
		border-radius: 6px;
		border: 1px solid var(--color-border-subtle);
		font-size: 0.8125rem;
	}

	.modal-confirm-section {
		border-top: 1px solid var(--color-border-subtle);
		padding-top: 1.25rem;
	}

	.modal-confirm-label {
		display: block;
		font-size: 0.8125rem;
		color: var(--color-text-secondary);
		margin-bottom: 0.5rem;
	}

	.modal-confirm-input {
		width: 100%;
		padding: 0.625rem 0.75rem;
		background: var(--color-bg-input, var(--color-bg-surface-hover));
		border: 1px solid var(--color-border);
		border-radius: 6px;
		color: var(--color-text-primary);
		font-size: 1rem;
		font-family: var(--font-mono);
		text-align: center;
		letter-spacing: 0.15em;
		outline: none;
		transition: border-color 150ms ease;
	}

	.modal-confirm-input:focus {
		border-color: var(--color-border-focus);
	}

	.modal-footer {
		padding: 1rem 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
	}
</style>
