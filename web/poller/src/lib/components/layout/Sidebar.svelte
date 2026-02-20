<script lang="ts">
	import { page } from '$app/state';
	import { NAV_ITEMS } from '$lib/constants';
	import { logout } from '$lib/stores/auth';

	interface Props {
		network?: string;
	}

	let { network = '' }: Props = $props();

	function isActive(path: string): boolean {
		if (path === '/') return page.url.pathname === '/';
		return page.url.pathname.startsWith(path);
	}
</script>

<aside class="sidebar">
	<!-- Brand -->
	<div class="sidebar-brand">
		<div class="brand-icon">P</div>
		<span>Poller</span>
	</div>

	<!-- Navigation -->
	<div class="sidebar-section">
		<div class="sidebar-label">DASHBOARD</div>
		<nav>
			<ul class="sidebar-nav">
				{#each NAV_ITEMS as item}
					<li>
						<a
							href={item.path}
							class="sidebar-item"
							class:active={isActive(item.path)}
						>
							{#if item.icon === 'chart'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18"/><path d="M9 21V9"/></svg>
							{:else if item.icon === 'list'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
							{:else if item.icon === 'eye'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
							{:else if item.icon === 'coins'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="8" cy="8" r="6"/><path d="M18.09 10.37A6 6 0 1 1 10.34 18"/><path d="M7 6h1v4"/><path d="m16.71 13.88.7.71-2.82 2.82"/></svg>
							{:else if item.icon === 'alert'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
							{:else if item.icon === 'settings'}
								<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-2 2 2 2 0 01-2-2v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 01-2-2 2 2 0 012-2h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 010-2.83 2 2 0 012.83 0l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 012-2 2 2 0 012 2v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 0 2 2 0 010 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 012 2 2 2 0 01-2 2h-.09a1.65 1.65 0 00-1.51 1z"/></svg>
							{/if}
							{item.label}
						</a>
					</li>
				{/each}
			</ul>
		</nav>
	</div>

	<!-- Footer -->
	<div class="sidebar-footer">
		<button class="sidebar-item logout-btn" onclick={logout}>
			<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
			Logout
		</button>
		{#if network}
			<div class="network-indicator">
				<span
					class="network-badge"
					class:network-testnet={network === 'testnet'}
					class:network-mainnet={network === 'mainnet'}
				>
					<span class="network-dot"></span>
					{network.toUpperCase()}
				</span>
			</div>
		{/if}
	</div>
</aside>

<style>
	.sidebar {
		width: var(--sidebar-width);
		background: var(--color-sidebar-bg);
		border-right: 1px solid var(--color-sidebar-border-color);
		display: flex;
		flex-direction: column;
		position: fixed;
		top: 0;
		left: 0;
		bottom: 0;
		z-index: 100;
		padding: 1rem 0;
		overflow-y: auto;
	}

	.sidebar-brand {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1.25rem;
		margin-bottom: 1rem;
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
	}

	.brand-icon {
		width: 28px;
		height: 28px;
		background: var(--color-accent-default);
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.8125rem;
		font-weight: 600;
		color: white;
	}

	.sidebar-section {
		margin-bottom: 1.5rem;
	}

	.sidebar-label {
		padding: 0.5rem 1.25rem;
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.025em;
	}

	.sidebar-nav {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.sidebar-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1.25rem;
		margin: 0 0.5rem;
		border-radius: 6px;
		color: var(--color-text-secondary);
		font-size: 0.875rem;
		font-weight: 400;
		cursor: pointer;
		transition: all 150ms ease;
		text-decoration: none;
		border: none;
		background: none;
		width: calc(100% - 1rem);
		text-align: left;
	}

	.sidebar-item:hover {
		background: var(--color-sidebar-hover);
		color: var(--color-text-primary);
	}

	.sidebar-item.active {
		background: var(--color-sidebar-active-bg);
		color: var(--color-text-primary);
		font-weight: 500;
	}

	.icon {
		width: 18px;
		height: 18px;
		opacity: 0.7;
		flex-shrink: 0;
	}

	.sidebar-item.active .icon {
		opacity: 1;
	}

	.sidebar-footer {
		margin-top: auto;
		padding: 1rem 1.25rem;
		border-top: 1px solid var(--color-sidebar-border-color);
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.logout-btn {
		font-family: var(--font-sans);
		margin: 0;
		width: 100%;
	}

	.network-indicator {
		display: flex;
		justify-content: center;
	}

	.network-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.25rem 0.5rem;
		border-radius: 9999px;
		font-size: 0.75rem;
		font-weight: 500;
	}

	.network-dot {
		width: 6px;
		height: 6px;
		background: currentColor;
		border-radius: 50%;
	}

	.network-testnet {
		background: var(--color-warning-muted);
		color: var(--color-warning);
	}

	.network-mainnet {
		background: var(--color-success-muted);
		color: var(--color-success);
	}
</style>
