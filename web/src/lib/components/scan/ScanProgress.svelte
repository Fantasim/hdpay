<script lang="ts">
	import { CHAIN_COLORS } from '$lib/constants';
	import type { Chain, ScanCompleteEvent, ScanErrorEvent, ScanProgress, ScanStateWithRunning } from '$lib/types';
	import { formatNumber, formatRelativeTime, parseElapsedToMs, formatDuration } from '$lib/utils/formatting';

	interface Props {
		chain: Chain;
		status: ScanStateWithRunning | null;
		progress: ScanProgress | null;
		lastComplete: ScanCompleteEvent | null;
		lastError: ScanErrorEvent | null;
	}

	let { chain, status, progress, lastComplete, lastError }: Props = $props();

	const chainLabels: Record<Chain, string> = {
		BTC: 'Bitcoin',
		BSC: 'BNB Chain',
		SOL: 'Solana'
	};

	let scanStatus = $derived(status?.status ?? 'idle');
	let isRunning = $derived(status?.isRunning === true);

	// Use isRunning to determine "scanning" display even when DB status hasn't updated yet.
	let displayStatus = $derived(isRunning ? 'scanning' : scanStatus);

	let progressPercent = $derived(
		progress ? Math.round((progress.scanned / progress.total) * 100) : (scanStatus === 'completed' ? 100 : 0)
	);

	let eta = $derived(() => {
		if (!progress || progress.scanned === 0) return '';
		// Parse elapsed string (e.g., "2m30s" or "5s" or "1h2m").
		const remaining = progress.total - progress.scanned;
		const elapsedMs = parseElapsedToMs(progress.elapsed);
		if (elapsedMs <= 0) return '';
		const ratePerMs = progress.scanned / elapsedMs;
		if (ratePerMs === 0) return '';
		const etaMs = remaining / ratePerMs;
		return formatDuration(etaMs);
	});

	let fundedCount = $derived(
		progress?.found ?? lastComplete?.found ?? status?.fundedCount ?? 0
	);
</script>

<div class="scan-item">
	<div class="scan-item-header">
		<div class="scan-item-chain">
			<span class="chain-dot" style="background: {CHAIN_COLORS[chain]}"></span>
			<span class="scan-item-chain-name">{chain}</span>
			<span class="badge badge-chain" style="background: {CHAIN_COLORS[chain]}20; color: {CHAIN_COLORS[chain]}">{chainLabels[chain]}</span>
		</div>
		{#if displayStatus === 'scanning'}
			<span class="badge badge-accent">Scanning</span>
		{:else if displayStatus === 'completed'}
			<span class="badge badge-success">Completed</span>
		{:else if displayStatus === 'failed'}
			<span class="badge badge-error">Error</span>
		{:else}
			<span class="badge badge-default">Idle</span>
		{/if}
	</div>

	<!-- Progress Bar -->
	{#if displayStatus === 'scanning' || displayStatus === 'completed'}
		<div class="progress">
			<div
				class="progress-bar"
				style="width: {progressPercent}%; background: {CHAIN_COLORS[chain]}"
			></div>
		</div>
	{/if}

	<!-- Details -->
	<div class="scan-item-details">
		{#if displayStatus === 'scanning' && progress}
			<span class="font-mono">{formatNumber(progress.scanned)} / {formatNumber(progress.total)}</span> scanned
			— <span class="text-success font-semibold">{progress.found}</span> funded
			{#if eta()}
				— ETA {eta()}
			{/if}
		{:else if displayStatus === 'completed' && lastComplete}
			<span class="font-mono">{formatNumber(lastComplete.scanned)} / {formatNumber(lastComplete.scanned)}</span> scanned
			— <span class="text-success font-semibold">{lastComplete.found}</span> funded
			— {lastComplete.duration}
		{:else if displayStatus === 'completed' && status}
			<span class="font-mono">{formatNumber(status.lastScannedIndex)} / {formatNumber(status.maxScanId)}</span> scanned
			— <span class="text-success font-semibold">{fundedCount}</span> funded
		{:else if displayStatus === 'failed' && lastError}
			<span class="text-error">{lastError.message}</span>
		{:else if status?.updatedAt}
			Last scan: {formatRelativeTime(status.updatedAt)}
			{#if fundedCount > 0}
				— <span class="text-success font-semibold">{fundedCount}</span> funded
			{/if}
		{:else}
			No scan history
		{/if}
	</div>
</div>

<style>
	.scan-item {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 1.25rem;
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 8px;
	}

	.scan-item-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.scan-item-chain {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}

	.chain-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.scan-item-chain-name {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.scan-item-details {
		font-size: 0.8125rem;
		color: var(--color-text-secondary);
	}

	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 9999px;
		font-size: 0.6875rem;
		font-weight: 500;
	}

	.badge-chain {
		border: none;
	}

	.badge-accent {
		background: var(--color-accent-muted);
		color: var(--color-accent-text);
	}

	.badge-success {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.badge-error {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.badge-default {
		background: var(--color-bg-surface-hover);
		color: var(--color-text-muted);
	}

	.progress {
		width: 100%;
		height: 6px;
		background: var(--color-bg-primary);
		border-radius: 3px;
		overflow: hidden;
	}

	.progress-bar {
		height: 100%;
		border-radius: 3px;
		transition: width 300ms ease;
	}

	.font-mono {
		font-family: var(--font-mono);
	}

	.font-semibold {
		font-weight: 600;
	}

	.text-success {
		color: var(--color-success);
	}

	.text-error {
		color: var(--color-error);
	}
</style>
