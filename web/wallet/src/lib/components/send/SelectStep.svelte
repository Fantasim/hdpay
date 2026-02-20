<script lang="ts">
	import { SUPPORTED_CHAINS, CHAIN_NATIVE_SYMBOLS, CHAIN_TOKENS } from '$lib/constants';
	import { sendStore } from '$lib/stores/send.svelte';
	import { getChainLabel } from '$lib/utils/chains';
	import type { Chain, SendToken } from '$lib/types';

	const store = sendStore;

	let selectedChain = $derived(store.state.chain);
	let selectedToken = $derived(store.state.token);
	let destination = $derived(store.state.destination);
	let destinationError = $derived(store.state.destinationError);
	let loading = $derived(store.state.loading);
	let error = $derived(store.state.error);

	// Available tokens for the selected chain.
	let availableTokens = $derived.by((): { value: SendToken; label: string }[] => {
		if (!selectedChain) return [];
		const native = CHAIN_NATIVE_SYMBOLS[selectedChain];
		const tokens: { value: SendToken; label: string }[] = [
			{ value: 'NATIVE', label: `${native} (Native)` }
		];
		const chainTokens = CHAIN_TOKENS[selectedChain];
		for (const t of chainTokens) {
			tokens.push({ value: t as SendToken, label: t });
		}
		return tokens;
	});

	let isValid = $derived(
		selectedChain !== null &&
		selectedToken !== null &&
		destination.trim().length > 0 &&
		destinationError === null
	);

	function handleChainSelect(chain: Chain): void {
		store.setChain(chain);
	}

	function handleTokenSelect(token: SendToken): void {
		store.setToken(token);
	}

	function handleDestinationInput(e: Event): void {
		const target = e.target as HTMLInputElement;
		store.setDestination(target.value);
	}

	async function handleContinue(): Promise<void> {
		await store.fetchPreview();
	}
</script>

<div class="card">
	<div class="card-header">
		<div class="card-title">Step 1: Select</div>
	</div>
	<div class="card-body">
		<!-- Chain Selector -->
		<div class="form-group">
			<span class="form-label">Chain</span>
			<div class="chain-selector" role="radiogroup" aria-label="Chain">
				{#each SUPPORTED_CHAINS as chain (chain)}
					<button
						class="chain-btn"
						class:active={selectedChain === chain}
						onclick={() => handleChainSelect(chain)}
					>
						<span class="chain-dot chain-dot-{chain.toLowerCase()}"></span>
						{getChainLabel(chain)}
					</button>
				{/each}
			</div>
		</div>

		<!-- Token Selector -->
		{#if selectedChain}
			<div class="form-group">
				<span class="form-label">Token</span>
				<div class="token-selector" role="radiogroup" aria-label="Token">
					{#each availableTokens as token (token.value)}
						<button
							class="token-btn"
							class:active={selectedToken === token.value}
							onclick={() => handleTokenSelect(token.value)}
						>
							{token.label}
						</button>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Destination Address -->
		{#if selectedToken}
			<div class="form-group">
				<label class="form-label" for="send-destination">Destination Address</label>
				<input
					id="send-destination"
					type="text"
					class="form-input"
					class:input-error={destinationError !== null && destination.length > 0}
					value={destination}
					oninput={handleDestinationInput}
					placeholder={selectedChain === 'BTC' ? 'bc1q... or tb1q...' : selectedChain === 'BSC' ? '0x...' : 'Base58 address...'}
				/>
				{#if destinationError && destination.length > 0}
					<div class="form-error">{destinationError}</div>
				{:else}
					<div class="form-hint">All funded addresses will be swept to this destination</div>
				{/if}
			</div>
		{/if}

		<!-- Error -->
		{#if error}
			<div class="alert alert-error">
				<svg class="alert-icon" viewBox="0 0 18 18" fill="none">
					<circle cx="9" cy="9" r="8" stroke="currentColor" stroke-width="1.5"/>
					<path d="M9 6v4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
					<circle cx="9" cy="13" r="0.5" fill="currentColor"/>
				</svg>
				{error}
			</div>
		{/if}

		<!-- Continue Button -->
		<div class="action-bar">
			<div></div>
			<button
				class="btn btn-primary"
				onclick={handleContinue}
				disabled={!isValid || loading}
			>
				{loading ? 'Loading Preview...' : 'Continue to Preview'}
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
					<path d="M6 3l5 5-5 5"/>
				</svg>
			</button>
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
	}

	.form-input:focus {
		border-color: var(--color-border-focus);
	}

	.form-input.input-error {
		border-color: var(--color-error);
	}

	.form-hint {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}

	.form-error {
		font-size: 0.6875rem;
		color: var(--color-error);
	}

	.chain-selector {
		display: flex;
		gap: 0.5rem;
	}

	.chain-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 150ms ease;
		border: 1px solid var(--color-border);
		background: var(--color-bg-input);
		color: var(--color-text-secondary);
	}

	.chain-btn:hover {
		border-color: var(--color-border-hover);
		color: var(--color-text-primary);
	}

	.chain-btn.active {
		border-color: var(--color-accent);
		background: var(--color-accent-muted);
		color: var(--color-text-primary);
	}

	.chain-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
	}

	.chain-dot-btc { background: var(--color-btc); }
	.chain-dot-bsc { background: var(--color-bsc); }
	.chain-dot-sol { background: var(--color-sol); }

	.token-selector {
		display: flex;
		gap: 0.5rem;
	}

	.token-btn {
		padding: 0.5rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 150ms ease;
		border: 1px solid var(--color-border);
		background: var(--color-bg-input);
		color: var(--color-text-secondary);
	}

	.token-btn:hover {
		border-color: var(--color-border-hover);
		color: var(--color-text-primary);
	}

	.token-btn.active {
		border-color: var(--color-accent);
		background: var(--color-accent-muted);
		color: var(--color-text-primary);
	}

	.alert {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		font-size: 0.8125rem;
		margin-bottom: 1rem;
	}

	.alert-error {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.alert-icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}

	.action-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 1.25rem;
		margin-top: 0.5rem;
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
</style>
