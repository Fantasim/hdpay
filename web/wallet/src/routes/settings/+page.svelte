<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import { getSettings, updateSettings, resetBalances } from '$lib/utils/api';
	import { RESUME_THRESHOLD_OPTIONS, LOG_LEVELS } from '$lib/constants';
	import type { Settings } from '$lib/types';

	// Settings state — local editable copy.
	let maxScanId = $state('5000');
	let autoResumeScans = $state(true);
	let resumeThresholdHours = $state('24');
	let btcFeeRate = $state('10');
	let bscGasPreseedBnb = $state('0.005');
	let logLevel = $state('info');
	let networkMode = $state<'mainnet' | 'testnet'>('testnet');

	let loading = $state(true);
	let saving = $state(false);
	let error: string | null = $state(null);
	let saveSuccess = $state(false);

	// Danger zone confirmation
	let confirmResetBalances = $state(false);
	let resetting = $state(false);

	async function loadSettings(): Promise<void> {
		loading = true;
		error = null;
		try {
			const res = await getSettings();
			const s = res.data as unknown as Settings;
			maxScanId = s.max_scan_id ?? '5000';
			autoResumeScans = s.auto_resume_scans === 'true';
			resumeThresholdHours = s.resume_threshold_hours ?? '24';
			btcFeeRate = s.btc_fee_rate ?? '10';
			bscGasPreseedBnb = s.bsc_gas_preseed_bnb ?? '0.005';
			logLevel = s.log_level ?? 'info';
			networkMode = (s.network === 'mainnet' ? 'mainnet' : 'testnet');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load settings';
		} finally {
			loading = false;
		}
	}

	async function handleSave(): Promise<void> {
		saving = true;
		error = null;
		saveSuccess = false;
		try {
			await updateSettings({
				max_scan_id: maxScanId,
				auto_resume_scans: String(autoResumeScans),
				resume_threshold_hours: resumeThresholdHours,
				btc_fee_rate: btcFeeRate,
				bsc_gas_preseed_bnb: bscGasPreseedBnb,
				log_level: logLevel,
			});
			saveSuccess = true;
			setTimeout(() => { saveSuccess = false; }, 2000);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to save settings';
		} finally {
			saving = false;
		}
	}

	async function handleResetBalances(): Promise<void> {
		if (!confirmResetBalances) {
			confirmResetBalances = true;
			return;
		}
		resetting = true;
		try {
			await resetBalances();
			confirmResetBalances = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reset balances';
		} finally {
			resetting = false;
		}
	}

	function cancelReset(): void {
		confirmResetBalances = false;
	}

	onMount(() => {
		loadSettings();
	});
</script>

<Header title="Settings" />
<p class="page-subtitle">Configure HDPay preferences</p>

{#if error}
	<div class="error-banner">{error}</div>
{/if}

{#if loading}
	<div class="loading-state">Loading settings...</div>
{:else}
	<div class="settings-body">
		<div class="settings-cards">

			<!-- Network Card (read-only, driven by HDPAY_NETWORK env var) -->
			<div class="card">
				<div class="card-header">
					<div class="card-title">Network</div>
				</div>
				<div class="card-body">
					<div class="form-group" style="margin-bottom: 0;">
						<label class="form-label">Active Network</label>
						<div class="network-badge">
							<span class="network-indicator network-indicator-{networkMode}"></span>
							{networkMode === 'mainnet' ? 'Mainnet' : 'Testnet'}
						</div>
						<div class="form-hint">Set via <code>HDPAY_NETWORK</code> environment variable</div>
					</div>
				</div>
			</div>

			<!-- Scanning Card -->
			<div class="card">
				<div class="card-header">
					<div class="card-title">Scanning</div>
				</div>
				<div class="card-body">
					<div class="form-group">
						<label class="form-label" for="max-scan-id">Default Max Scan ID</label>
						<input
							id="max-scan-id"
							type="number"
							class="form-input"
							bind:value={maxScanId}
							style="max-width: 200px;"
						/>
						<div class="form-hint">Addresses 0 through this value will be scanned per chain</div>
					</div>

					<div class="toggle-row">
						<div class="toggle-info">
							<div class="toggle-label">Auto-resume scans</div>
							<div class="toggle-desc">Automatically resume interrupted scans on startup</div>
						</div>
						<button
							class="toggle-switch"
							class:active={autoResumeScans}
							onclick={() => { autoResumeScans = !autoResumeScans; }}
						>
							<div class="toggle-switch-knob"></div>
						</button>
					</div>

					<div class="form-group" style="margin-top: 1rem; margin-bottom: 0;">
						<label class="form-label" for="resume-threshold">Resume threshold</label>
						<select id="resume-threshold" class="form-select" bind:value={resumeThresholdHours} style="max-width: 200px;">
							{#each RESUME_THRESHOLD_OPTIONS as hours}
								<option value={String(hours)}>{hours} {hours === 1 ? 'hour' : 'hours'}</option>
							{/each}
						</select>
						<div class="form-hint">Scans older than this threshold will restart from the beginning</div>
					</div>
				</div>
			</div>

			<!-- Transaction Card -->
			<div class="card">
				<div class="card-header">
					<div class="card-title">Transaction</div>
				</div>
				<div class="card-body">
					<div class="form-row">
						<div class="form-group" style="margin-bottom: 0;">
							<label class="form-label" for="btc-fee">BTC Fee Rate</label>
							<div class="input-with-suffix">
								<input id="btc-fee" type="number" class="form-input" bind:value={btcFeeRate} />
								<span class="input-suffix">sat/vB</span>
							</div>
							<div class="form-hint">Set to 0 for dynamic fee estimation</div>
						</div>
						<div class="form-group" style="margin-bottom: 0;">
							<label class="form-label" for="gas-preseed">BSC Gas Pre-Seed Amount</label>
							<div class="input-with-suffix">
								<input id="gas-preseed" type="number" class="form-input" bind:value={bscGasPreseedBnb} step="0.001" />
								<span class="input-suffix">BNB</span>
							</div>
							<div class="form-hint">Amount of BNB sent to each address needing gas</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Display Card -->
			<div class="card">
				<div class="card-header">
					<div class="card-title">Display</div>
				</div>
				<div class="card-body">
					<div class="form-row">
						<div class="form-group" style="margin-bottom: 0;">
							<label class="form-label" for="log-level">Log Level</label>
							<select id="log-level" class="form-select" bind:value={logLevel}>
								{#each LOG_LEVELS as level}
									<option value={level}>{level.charAt(0).toUpperCase() + level.slice(1)}</option>
								{/each}
							</select>
						</div>
						<div class="form-group" style="margin-bottom: 0;">
							<label class="form-label">
								Price Currency
								<span class="coming-soon">Coming soon</span>
							</label>
							<select class="form-select" disabled>
								<option selected>USD</option>
								<option>EUR</option>
								<option>GBP</option>
							</select>
						</div>
					</div>
				</div>
			</div>

			<!-- Danger Zone Card -->
			<div class="card card-danger">
				<div class="card-header">
					<div class="card-title danger-title">Danger Zone</div>
				</div>
				<div class="card-body">
					<div class="danger-item">
						<div class="danger-item-info">
							<div class="danger-item-title">Reset Database</div>
							<div class="danger-item-desc">Delete all scan results and transaction history. Addresses will be preserved.</div>
						</div>
						{#if confirmResetBalances}
							<div class="confirm-group">
								<button class="btn btn-danger btn-sm" onclick={handleResetBalances} disabled={resetting}>
									{resetting ? 'Resetting...' : 'Confirm Reset'}
								</button>
								<button class="btn btn-secondary btn-sm" onclick={cancelReset}>Cancel</button>
							</div>
						{:else}
							<button class="btn btn-danger" onclick={handleResetBalances}>Reset</button>
						{/if}
					</div>
				</div>
			</div>

			<!-- Save Footer -->
			<div class="settings-footer">
				<button class="btn btn-primary btn-lg" onclick={handleSave} disabled={saving}>
					{#if saving}
						Saving...
					{:else if saveSuccess}
						Saved!
					{:else}
						Save Settings
					{/if}
				</button>
			</div>

		</div>
	</div>
{/if}

<style>
	.page-subtitle {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		margin: -1rem 0 1.5rem 0;
	}

	.settings-body {
		max-width: 680px;
	}

	.settings-cards {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	/* Cards */
	.card {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border);
		border-radius: 8px;
	}

	.card-header {
		padding: 1rem 1.25rem 0;
	}

	.card-title {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.card-body {
		padding: 1rem 1.25rem 1.25rem;
	}

	.card-danger {
		border-color: rgba(239, 68, 68, 0.25);
	}

	.danger-title {
		color: var(--color-error);
	}

	/* Network badge (read-only) */
	.network-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.5rem 0.875rem;
		background: var(--color-bg-input, var(--color-bg-surface));
		border: 1px solid var(--color-border);
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.network-indicator {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.network-indicator-mainnet { background: var(--color-success); }
	.network-indicator-testnet { background: var(--color-warning); }

	/* Form elements */
	.form-group {
		margin-bottom: 1rem;
	}

	.form-label {
		display: block;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
		margin-bottom: 0.375rem;
	}

	.form-input, .form-select {
		display: block;
		height: 36px;
		padding: 0 0.75rem;
		background: var(--color-bg-input, var(--color-bg-surface));
		border: 1px solid var(--color-border);
		border-radius: 6px;
		font-size: 0.8125rem;
		color: var(--color-text-primary);
		transition: border-color 150ms ease;
	}

	.form-input:focus, .form-select:focus {
		outline: none;
		border-color: var(--color-accent);
	}

	.form-hint {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		margin-top: 0.375rem;
	}

	.form-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
		align-items: start;
	}

	/* Input with suffix */
	.input-with-suffix {
		display: flex;
		align-items: center;
	}

	.input-with-suffix .form-input {
		border-top-right-radius: 0;
		border-bottom-right-radius: 0;
		border-right: none;
	}

	.input-suffix {
		display: flex;
		align-items: center;
		height: 36px;
		padding: 0 0.75rem;
		background: var(--color-bg-surface-active, var(--color-bg-surface));
		border: 1px solid var(--color-border);
		border-left: none;
		border-radius: 0 6px 6px 0;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	/* Toggle switch */
	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.75rem 0;
	}

	.toggle-info {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.toggle-label {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.toggle-desc {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}

	.toggle-switch {
		width: 40px;
		height: 22px;
		background: var(--color-border);
		border-radius: 9999px;
		position: relative;
		cursor: pointer;
		transition: background 150ms ease;
		flex-shrink: 0;
		border: none;
	}

	.toggle-switch.active {
		background: var(--color-accent);
	}

	.toggle-switch-knob {
		width: 16px;
		height: 16px;
		background: white;
		border-radius: 50%;
		position: absolute;
		top: 3px;
		left: 3px;
		transition: transform 150ms ease;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
	}

	.toggle-switch.active .toggle-switch-knob {
		transform: translateX(18px);
	}

	/* Coming soon */
	.coming-soon {
		font-size: 0.6875rem;
		color: var(--color-text-disabled);
		font-style: italic;
		margin-left: 0.5rem;
		font-weight: 400;
	}

	/* Danger zone */
	.danger-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1rem 0;
	}

	.danger-item + .danger-item {
		border-top: 1px solid var(--color-border-subtle);
	}

	.danger-item-info {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.danger-item-title {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.danger-item-desc {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		max-width: 420px;
	}

	.confirm-group {
		display: flex;
		gap: 0.5rem;
	}

	/* Buttons */
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
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--color-accent);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		filter: brightness(1.1);
	}

	.btn-secondary {
		background: var(--color-bg-surface);
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--color-bg-surface-hover);
	}

	.btn-danger {
		background: var(--color-error);
		color: white;
	}

	.btn-danger:hover:not(:disabled) {
		filter: brightness(1.1);
	}

	.btn-sm {
		padding: 0.375rem 0.75rem;
		font-size: 0.75rem;
	}

	.btn-lg {
		padding: 0.625rem 1.5rem;
		font-size: 0.875rem;
	}

	/* Save footer */
	.settings-footer {
		padding-top: 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
		margin-top: 0.5rem;
	}

	/* States */
	.loading-state {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 300px;
		border: 1px dashed var(--color-border);
		border-radius: 8px;
		color: var(--color-text-muted);
		font-size: 0.875rem;
	}

	.error-banner {
		margin-bottom: 1rem;
		padding: 0.75rem 1rem;
		border-radius: 6px;
		background: var(--color-error-muted);
		color: var(--color-error);
		font-size: 0.8125rem;
	}

</style>
