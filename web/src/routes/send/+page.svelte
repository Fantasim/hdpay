<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import SelectStep from '$lib/components/send/SelectStep.svelte';
	import PreviewStep from '$lib/components/send/PreviewStep.svelte';
	import GasPreSeedStep from '$lib/components/send/GasPreSeedStep.svelte';
	import ExecuteStep from '$lib/components/send/ExecuteStep.svelte';
	import { sendStore } from '$lib/stores/send.svelte';
	import { CHAIN_NATIVE_SYMBOLS } from '$lib/constants';
	import type { SendStep } from '$lib/types';

	const store = sendStore;

	let currentStep = $derived(store.state.step);
	let chain = $derived(store.state.chain);
	let preview = $derived(store.state.preview);

	const STEPS: { key: SendStep; label: string; number: number }[] = [
		{ key: 'select', label: 'Select', number: 1 },
		{ key: 'preview', label: 'Preview', number: 2 },
		{ key: 'gas-preseed', label: 'Gas Pre-Seed', number: 3 },
		{ key: 'execute', label: 'Execute', number: 4 }
	];

	const STEP_ORDER: SendStep[] = ['select', 'preview', 'gas-preseed', 'execute', 'complete'];

	function stepIndex(step: SendStep): number {
		return STEP_ORDER.indexOf(step);
	}

	function isStepCompleted(stepKey: SendStep): boolean {
		return stepIndex(currentStep) > stepIndex(stepKey);
	}

	function isStepActive(stepKey: SendStep): boolean {
		if (stepKey === 'execute') {
			return currentStep === 'execute' || currentStep === 'complete';
		}
		return currentStep === stepKey;
	}

	let tokenLabel = $derived(
		preview?.token === 'NATIVE' && chain
			? CHAIN_NATIVE_SYMBOLS[chain]
			: preview?.token ?? store.state.token ?? ''
	);

	let selectSummary = $derived.by((): string => {
		if (!chain) return '';
		const tok = tokenLabel || 'NATIVE';
		const count = preview?.fundedCount;
		return count
			? `${chain} / ${tok} — ${count} funded addresses`
			: `${chain} / ${tok}`;
	});

	onMount(() => {
		return () => {
			// Don't reset on unmount — user might navigate back.
		};
	});
</script>

<Header title="Send">
	{#snippet actions()}
		<span class="subtitle">Sweep funded addresses to a consolidation destination</span>
	{/snippet}
</Header>

<div class="wizard">
	<!-- Stepper -->
	<div class="stepper">
		{#each STEPS as step, i (step.key)}
			{#if i > 0}
				<div class="stepper-separator" class:completed={isStepCompleted(STEPS[i - 1].key)}></div>
			{/if}
			<div
				class="stepper-step"
				class:completed={isStepCompleted(step.key)}
				class:active={isStepActive(step.key)}
			>
				<div class="stepper-number">
					{#if isStepCompleted(step.key)}
						<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<path d="M3.5 8.5l3 3 6-6"/>
						</svg>
					{:else}
						{step.number}
					{/if}
				</div>
				<span class="stepper-label">{step.label}</span>
			</div>
		{/each}
	</div>

	<!-- Collapsed completed steps -->
	{#if isStepCompleted('select') && currentStep !== 'select'}
		<button class="step-collapsed" onclick={() => store.goToStep('select')}>
			<div class="step-collapsed-icon">
				<svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="var(--color-success)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M3.5 8.5l3 3 6-6"/>
				</svg>
			</div>
			<span class="step-collapsed-title">Step 1: Select</span>
			<span class="step-collapsed-summary">
				{#if chain}
					<span class="badge badge-{chain.toLowerCase()}">{chain}</span>
				{/if}
				{selectSummary}
			</span>
		</button>
	{/if}

	<!-- Step Content -->
	{#if currentStep === 'select'}
		<SelectStep />
	{:else if currentStep === 'preview'}
		<PreviewStep />
	{:else if currentStep === 'gas-preseed'}
		<GasPreSeedStep />
	{:else if currentStep === 'execute' || currentStep === 'complete'}
		<ExecuteStep />
	{/if}
</div>

<style>
	.wizard {
		max-width: 720px;
	}

	.subtitle {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
	}

	/* Stepper */
	.stepper {
		display: flex;
		align-items: center;
		gap: 0;
		margin-bottom: 1.5rem;
		padding: 1rem 0;
	}

	.stepper-step {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.stepper-number {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.6875rem;
		font-weight: 600;
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border);
		color: var(--color-text-muted);
	}

	.stepper-step.active .stepper-number {
		background: var(--color-accent);
		border-color: var(--color-accent);
		color: white;
	}

	.stepper-step.completed .stepper-number {
		background: var(--color-success-muted);
		border-color: var(--color-success);
		color: var(--color-success);
	}

	.stepper-label {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
	}

	.stepper-step.active .stepper-label {
		color: var(--color-text-primary);
	}

	.stepper-step.completed .stepper-label {
		color: var(--color-text-secondary);
	}

	.stepper-separator {
		flex: 1;
		height: 1px;
		background: var(--color-border);
		margin: 0 0.75rem;
		min-width: 24px;
	}

	.stepper-separator.completed {
		background: var(--color-success);
	}

	/* Collapsed step */
	.step-collapsed {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
		padding: 0.75rem 1rem;
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
		margin-bottom: 1rem;
		cursor: pointer;
		transition: background 150ms ease;
		font-family: inherit;
		font-size: inherit;
		text-align: left;
	}

	.step-collapsed:hover {
		background: var(--color-bg-surface-hover);
	}

	.step-collapsed-icon {
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: var(--color-success-muted);
		border: 1px solid var(--color-success);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.step-collapsed-title {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-secondary);
	}

	.step-collapsed-summary {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		margin-left: auto;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

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
</style>
