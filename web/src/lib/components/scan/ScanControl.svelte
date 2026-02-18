<script lang="ts">
	import { scanStore } from '$lib/stores/scan.svelte';
	import { DEFAULT_MAX_SCAN_ID, MAX_SCAN_ID, SUPPORTED_CHAINS } from '$lib/constants';
	import type { Chain } from '$lib/types';
	import { formatNumber } from '$lib/utils/formatting';

	let selectedChain: Chain = $state('BTC');
	let maxId: number = $state(DEFAULT_MAX_SCAN_ID);
	let starting = $state(false);

	let isRunning = $derived(
		scanStore.state.statuses[selectedChain]?.isRunning === true
	);

	let anyRunning = $derived(scanStore.isAnyScanning());

	let estimateMinutes = $derived(Math.max(1, Math.ceil(maxId / 1000)));

	async function handleStart(): Promise<void> {
		starting = true;
		try {
			await scanStore.startScan(selectedChain, maxId);
		} catch {
			// Error is already set in the store.
		} finally {
			starting = false;
		}
	}

	function handleStop(): void {
		scanStore.stopScan(selectedChain);
	}
</script>

<div class="card">
	<div class="card-header">
		<div class="card-title">Scan Control</div>
	</div>
	<div class="card-body">
		<div class="scan-controls">
			<!-- Chain Selector -->
			<div class="form-group">
				<label class="form-label" for="scan-chain">Chain</label>
				<select
					id="scan-chain"
					class="form-select"
					bind:value={selectedChain}
				>
					{#each SUPPORTED_CHAINS as chain}
						<option value={chain}>{chain}</option>
					{/each}
				</select>
			</div>

			<!-- Max ID Input -->
			<div class="form-group">
				<label class="form-label" for="scan-maxid">Max ID</label>
				<input
					id="scan-maxid"
					type="number"
					class="form-input"
					bind:value={maxId}
					min="1"
					max={MAX_SCAN_ID}
					placeholder={String(DEFAULT_MAX_SCAN_ID)}
				/>
				<div class="form-hint">Scan addresses 0 through this ID</div>
			</div>

			<!-- Start Button -->
			<div class="form-group form-group-btn">
				<button
					class="btn btn-primary"
					onclick={handleStart}
					disabled={isRunning || starting}
				>
					<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="7" cy="7" r="5"/>
						<path d="M11 11l3 3"/>
					</svg>
					{starting ? 'Starting...' : 'Start Scan'}
				</button>
			</div>

			<!-- Stop Button -->
			<div class="form-group form-group-btn">
				<button
					class="btn btn-danger"
					onclick={handleStop}
					disabled={!isRunning}
				>
					<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
						<rect x="4" y="4" width="8" height="8" rx="1"/>
					</svg>
					Stop
				</button>
			</div>
		</div>

		<!-- Info Alert -->
		<div class="alert alert-info">
			<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
				<circle cx="9" cy="9" r="8" stroke="currentColor" stroke-width="1.5"/>
				<path d="M9 8v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
				<path d="M9 6.5v0" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
			</svg>
			Scan will check addresses 0 through {formatNumber(maxId)}. Estimated time: ~{estimateMinutes} minute{estimateMinutes !== 1 ? 's' : ''}.
		</div>
	</div>
</div>

<style>
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
		margin-bottom: 2rem;
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

	.scan-controls {
		display: grid;
		grid-template-columns: 1fr 1fr auto auto;
		gap: 1rem;
		align-items: end;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.form-group-btn {
		padding-top: 1.25rem;
	}

	.form-label {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.form-select,
	.form-input {
		padding: 0.5rem 0.75rem;
		background: var(--color-bg-input);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		color: var(--color-text-primary);
		font-size: 0.8125rem;
		font-family: inherit;
		outline: none;
		transition: border-color 150ms ease;
	}

	.form-select:focus,
	.form-input:focus {
		border-color: var(--color-border-focus);
	}

	.form-hint {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
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

	.btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--color-accent);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--color-accent-hover);
	}

	.btn-danger {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.btn-danger:hover:not(:disabled) {
		background: var(--color-error);
		color: white;
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

	.alert-info {
		background: var(--color-info-muted);
		color: var(--color-info);
	}

	.alert-icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}
</style>
