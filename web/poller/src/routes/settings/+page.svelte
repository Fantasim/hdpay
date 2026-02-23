<script lang="ts">
	import { onMount } from 'svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import {
		getSettings,
		getHealth,
		updateTiers,
		updateWatchDefaults,
		getAllowlist,
		addAllowlistIP,
		removeAllowlistIP
	} from '$lib/utils/api';
	import { formatNumber } from '$lib/utils/formatting';
	import type {
		AdminSettings,
		HealthResponse,
		Tier,
		IPAllowlistEntry
	} from '$lib/types';

	// Data state
	let settings: AdminSettings | null = $state(null);
	let health: HealthResponse | null = $state(null);
	let allowlist: IPAllowlistEntry[] = $state([]);
	let loading = $state(true);

	// Editable tier state
	let editTiers: Tier[] = $state([]);

	// Watch defaults
	let watchTimeout = $state(30);
	let maxActiveWatches = $state(100);

	// Add IP state
	let newIp = $state('');
	let newIpDesc = $state('');

	// Save feedback
	let tiersSaved = $state(false);
	let defaultsSaved = $state(false);

	async function fetchData(): Promise<void> {
		loading = true;
		try {
			const [settingsRes, healthRes, allowlistRes] = await Promise.all([
				getSettings(),
				getHealth(),
				getAllowlist()
			]);
			settings = settingsRes.data;
			health = healthRes.data;
			allowlist = allowlistRes.data ?? [];

			// Initialize editable state
			editTiers = settings.tiers.map((t) => ({ ...t }));
			watchTimeout = settings.default_watch_timeout_min;
			maxActiveWatches = settings.max_active_watches;
		} catch (err) {
			console.error('Failed to fetch settings', err);
		} finally {
			loading = false;
		}
	}

	async function saveTiers(): Promise<void> {
		try {
			await updateTiers(editTiers);
			tiersSaved = true;
			setTimeout(() => {
				tiersSaved = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to save tiers', err);
		}
	}

	async function saveDefaults(): Promise<void> {
		try {
			await updateWatchDefaults({
				default_watch_timeout_min: watchTimeout,
				max_active_watches: maxActiveWatches
			});
			defaultsSaved = true;
			setTimeout(() => {
				defaultsSaved = false;
			}, 2000);
		} catch (err) {
			console.error('Failed to save watch defaults', err);
		}
	}

	async function handleAddIP(): Promise<void> {
		if (!newIp.trim()) return;
		try {
			const res = await addAllowlistIP(newIp.trim(), newIpDesc.trim() || undefined);
			allowlist = [...allowlist, res.data];
			newIp = '';
			newIpDesc = '';
		} catch (err) {
			console.error('Failed to add IP', err);
		}
	}

	async function handleRemoveIP(id: number): Promise<void> {
		try {
			await removeAllowlistIP(id);
			allowlist = allowlist.filter((e) => e.id !== id);
		} catch (err) {
			console.error('Failed to remove IP', err);
		}
	}

	function tierExample(tier: Tier): string {
		const midUsd = tier.max_usd !== null
			? (tier.min_usd + tier.max_usd) / 2
			: tier.min_usd * 2;
		const pts = Math.round(midUsd * 100 * tier.multiplier);
		return `$${midUsd.toFixed(2)} \u2192 ${formatNumber(pts)} pts`;
	}

	onMount(() => {
		fetchData();
	});
</script>

<svelte:head>
	<title>Poller — Settings</title>
</svelte:head>

<Header title="Settings" subtitle="Configuration and system management" />

<div class="page-body">
	{#if loading && !settings}
		<div class="loading-state">Loading settings...</div>
	{:else if settings}
		<!-- Section 1: Points Tier Editor -->
		<div class="settings-section">
			<div class="settings-section-title">Points Tiers</div>
			<div class="settings-section-desc">
				Configure USD-to-points conversion tiers. Changes apply to new transactions only.
			</div>

			<div class="table-wrapper">
				<table class="table">
					<thead>
						<tr>
							<th>Tier</th>
							<th>Min USD</th>
							<th>Max USD</th>
							<th>Multiplier</th>
							<th>Example</th>
						</tr>
					</thead>
					<tbody>
						{#each editTiers as tier, i}
							<tr class:tier-row-zero={tier.multiplier === 0}>
								<td><span class="tier-number">{i}</span></td>
								<td>
									<input
										class="tier-input"
										type="number"
										step="0.01"
										min="0"
										bind:value={tier.min_usd}
									/>
								</td>
								<td>
									{#if tier.max_usd !== null}
										<input
											class="tier-input"
											type="number"
											step="0.01"
											min="0"
											bind:value={tier.max_usd}
										/>
									{:else}
										<input
											class="tier-input"
											type="text"
											value={'\u221E'}
											disabled
											style="opacity: 0.5; cursor: not-allowed;"
										/>
									{/if}
								</td>
								<td>
									<input
										class="tier-input"
										type="number"
										step="0.1"
										min="0"
										bind:value={tier.multiplier}
									/>
								</td>
								<td class="text-muted text-sm">{tierExample(tier)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<div class="section-actions">
				<button class="btn btn-primary" type="button" onclick={saveTiers}>
					{tiersSaved ? 'Saved!' : 'Save Tiers'}
				</button>
			</div>
		</div>

		<hr class="section-divider" />

		<!-- Section 2: IP Allowlist -->
		<div class="settings-section">
			<div class="settings-section-title">IP Allowlist</div>
			<div class="settings-section-desc">
				Manage which external IPs can access the API. Localhost and private networks are always
				allowed.
			</div>

			{#if allowlist.length > 0}
				<div class="table-wrapper">
					<table class="table">
						<thead>
							<tr>
								<th>IP Address</th>
								<th>Description</th>
								<th>Added</th>
								<th>Actions</th>
							</tr>
						</thead>
						<tbody>
							{#each allowlist as entry}
								<tr>
									<td class="ip-cell">{entry.ip}</td>
									<td class="text-secondary">{entry.description ?? '\u2014'}</td>
									<td class="text-muted">{entry.added_at.split('T')[0]}</td>
									<td>
										<button
											class="btn btn-sm btn-danger"
											type="button"
											onclick={() => handleRemoveIP(entry.id)}
										>
											Remove
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="empty-hint">No IPs in allowlist.</div>
			{/if}

			<div class="add-ip-row">
				<input
					class="form-input form-input-ip"
					type="text"
					placeholder="203.0.113.50"
					bind:value={newIp}
				/>
				<input
					class="form-input"
					type="text"
					placeholder="Optional label"
					bind:value={newIpDesc}
				/>
				<button class="btn btn-sm btn-primary" type="button" onclick={handleAddIP}>
					Add
				</button>
			</div>
		</div>

		<hr class="section-divider" />

		<!-- Section 3: Watch Defaults -->
		<div class="settings-section">
			<div class="settings-section-title">Watch Defaults</div>
			<div class="settings-section-desc">Default values for new watches.</div>

			<div class="form-row-2col">
				<div class="form-group">
					<label class="form-label" for="watch-timeout">Default Timeout (minutes)</label>
					<input
						class="form-input"
						id="watch-timeout"
						type="number"
						min="1"
						max="120"
						bind:value={watchTimeout}
					/>
					<div class="form-hint">How long watches stay active (1-120 min)</div>
				</div>
				<div class="form-group">
					<label class="form-label" for="max-watches">Max Active Watches</label>
					<input
						class="form-input"
						id="max-watches"
						type="number"
						min="1"
						bind:value={maxActiveWatches}
					/>
					<div class="form-hint">Maximum concurrent watches allowed</div>
				</div>
			</div>

			<div class="section-actions">
				<button class="btn btn-primary" type="button" onclick={saveDefaults}>
					{defaultsSaved ? 'Saved!' : 'Save Defaults'}
				</button>
			</div>
		</div>

		<hr class="section-divider" />

		<!-- Section 4: System Information -->
		<div class="settings-section">
			<div class="settings-section-title">System Information</div>
			<div class="settings-section-desc">Read-only system status.</div>

			<div class="sysinfo-grid">
				{#if health}
					<div class="sysinfo-item">
						<div class="sysinfo-label">Uptime</div>
						<div class="sysinfo-value mono">{health.uptime}</div>
					</div>
					<div class="sysinfo-item">
						<div class="sysinfo-label">Version</div>
						<div class="sysinfo-value mono">{health.version}</div>
					</div>
				{/if}
				<div class="sysinfo-item">
					<div class="sysinfo-label">Network</div>
					<div class="sysinfo-value">
						<span
							class="network-badge"
							class:network-testnet={settings.network === 'testnet'}
							class:network-mainnet={settings.network === 'mainnet'}
						>
							{settings.network.toUpperCase()}
						</span>
					</div>
				</div>
				<div class="sysinfo-item">
					<div class="sysinfo-label">Database Path</div>
					<div class="sysinfo-value mono">{settings.db_path}</div>
				</div>
				<div class="sysinfo-item">
					<div class="sysinfo-label">Start Date</div>
					<div class="sysinfo-value mono">
						{new Date(settings.start_date * 1000).toISOString().split('T')[0]}
					</div>
				</div>
				<div class="sysinfo-item">
					<div class="sysinfo-label">Tiers File</div>
					<div class="sysinfo-value mono">{settings.tiers_file}</div>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.page-body {
		padding: 1.5rem 2rem;
	}

	.loading-state {
		color: var(--color-text-muted);
		font-size: 0.875rem;
		padding: 2rem 0;
	}

	/* Section layout */
	.settings-section {
		margin-bottom: 0.5rem;
	}

	.settings-section-title {
		font-size: 0.9375rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin-bottom: 0.25rem;
	}

	.settings-section-desc {
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		margin-bottom: 1rem;
	}

	.section-divider {
		border: none;
		border-top: 1px solid var(--color-border-subtle);
		margin: 1.5rem 0;
	}

	.section-actions {
		margin-top: 1rem;
	}

	/* Table */
	.table-wrapper {
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
		overflow: hidden;
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8125rem;
	}

	.table thead {
		background: var(--color-bg-elevated);
	}

	.table th {
		padding: 0.625rem 0.75rem;
		text-align: left;
		font-weight: 500;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		white-space: nowrap;
		border-bottom: 1px solid var(--color-border-subtle);
	}

	.table td {
		padding: 0.625rem 0.75rem;
		border-bottom: 1px solid var(--color-border-subtle);
		vertical-align: middle;
	}

	.table tbody tr:hover {
		background: var(--color-bg-surface-hover);
	}

	.table tbody tr:last-child td {
		border-bottom: none;
	}

	/* Tier editor */
	.tier-input {
		width: 80px;
		height: 30px;
		padding: 0.125rem 0.5rem;
		background: var(--color-bg-input);
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		font-family: var(--font-mono);
		font-size: 0.6875rem;
		color: var(--color-text-primary);
		text-align: right;
		transition: border-color 0.15s ease;
	}

	.tier-input:focus {
		outline: none;
		border-color: var(--color-border-focus);
	}

	/* Hide number input spinners */
	.tier-input::-webkit-outer-spin-button,
	.tier-input::-webkit-inner-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}

	.tier-input {
		-moz-appearance: textfield;
	}

	.tier-number {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		border-radius: var(--radius-sm);
		background: var(--color-bg-surface-active);
		font-size: 0.6875rem;
		font-weight: 500;
		color: var(--color-text-muted);
		font-family: var(--font-mono);
	}

	.tier-row-zero td {
		opacity: 0.55;
	}

	/* IP Allowlist */
	.ip-cell {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
		color: var(--color-accent-text);
	}

	.empty-hint {
		color: var(--color-text-muted);
		font-size: 0.8125rem;
		padding: 0.5rem 0;
	}

	.add-ip-row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-top: 1rem;
	}

	.form-input-ip {
		width: 200px;
		flex: none;
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	/* Watch Defaults */
	.form-row-2col {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.5rem;
		margin-bottom: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.form-label {
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.form-input {
		background: var(--color-bg-input);
		border: 1px solid var(--color-border-default);
		border-radius: var(--radius-md);
		color: var(--color-text-primary);
		font-family: var(--font-sans);
		font-size: 0.8125rem;
		padding: 0.5rem 0.75rem;
	}

	.form-input:focus {
		border-color: var(--color-border-focus);
		outline: none;
	}

	.form-hint {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}

	/* System Info */
	.sysinfo-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}

	.sysinfo-item {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		padding: 0.875rem;
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: var(--radius-lg);
	}

	.sysinfo-label {
		font-size: 0.6875rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.sysinfo-value {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--color-text-primary);
	}

	.sysinfo-value.mono {
		font-family: var(--font-mono);
		font-size: 0.8125rem;
	}

	/* Helpers */
	.text-muted {
		color: var(--color-text-muted);
	}

	.text-secondary {
		color: var(--color-text-secondary);
	}

	.text-sm {
		font-size: 0.8125rem;
	}

	/* Network badge */
	.network-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.125rem 0.5rem;
		border-radius: var(--radius-md);
		font-size: 0.6875rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.network-testnet {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.network-mainnet {
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	/* Buttons */
	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: none;
		border-radius: var(--radius-md);
		font-family: var(--font-sans);
		font-weight: 500;
		cursor: pointer;
		transition: all 0.15s ease;
	}

	.btn-primary {
		background: var(--color-accent-default);
		color: white;
		padding: 0.5rem 1rem;
		font-size: 0.8125rem;
	}

	.btn-primary:hover {
		background: var(--color-accent-hover);
	}

	.btn-sm {
		padding: 0.25rem 0.75rem;
		font-size: 0.75rem;
	}

	.btn-danger {
		background: var(--color-error-muted);
		color: var(--color-error);
	}

	.btn-danger:hover {
		background: rgba(239, 68, 68, 0.25);
	}
</style>
