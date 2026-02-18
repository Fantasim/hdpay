<script lang="ts">
	import { page } from '$app/state';

	interface NavItem {
		label: string;
		href: string;
		icon: string;
	}

	const mainNav: NavItem[] = [
		{
			label: 'Dashboard',
			href: '/',
			icon: '<rect x="2" y="2" width="6" height="6" rx="1"/><rect x="10" y="2" width="6" height="6" rx="1"/><rect x="2" y="10" width="6" height="6" rx="1"/><rect x="10" y="10" width="6" height="6" rx="1"/>'
		},
		{
			label: 'Addresses',
			href: '/addresses',
			icon: '<path d="M3 5h12M3 9h12M3 13h8"/>'
		},
		{
			label: 'Scan',
			href: '/scan',
			icon: '<circle cx="8" cy="8" r="5"/><path d="M13 13l3 3"/>'
		},
		{
			label: 'Send',
			href: '/send',
			icon: '<path d="M3 9h12M12 6l3 3-3 3"/>'
		},
		{
			label: 'Transactions',
			href: '/transactions',
			icon: '<path d="M3 4h12M3 8h12M3 12h12M3 16h8"/>'
		}
	];

	const systemNav: NavItem[] = [
		{
			label: 'Settings',
			href: '/settings',
			icon: '<circle cx="9" cy="9" r="2.5"/><path d="M9 2v2M9 14v2M2 9h2M14 9h2M4.2 4.2l1.4 1.4M12.4 12.4l1.4 1.4M4.2 13.8l1.4-1.4M12.4 5.6l1.4-1.4"/>'
		}
	];

	function isActive(href: string): boolean {
		if (href === '/') return page.url.pathname === '/';
		return page.url.pathname.startsWith(href);
	}
</script>

<nav class="sidebar">
	<div class="sidebar-brand">
		<div class="brand-icon">H</div>
		<span class="brand-text">HDPay</span>
	</div>

	<div class="sidebar-section">
		<ul class="sidebar-nav">
			{#each mainNav as item}
				<li>
					<a
						href={item.href}
						class="sidebar-item"
						class:active={isActive(item.href)}
					>
						<svg class="icon" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
							{@html item.icon}
						</svg>
						{item.label}
					</a>
				</li>
			{/each}
		</ul>
	</div>

	<div class="sidebar-section">
		<div class="sidebar-label">System</div>
		<ul class="sidebar-nav">
			{#each systemNav as item}
				<li>
					<a
						href={item.href}
						class="sidebar-item"
						class:active={isActive(item.href)}
					>
						<svg class="icon" viewBox="0 0 18 18" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
							{@html item.icon}
						</svg>
						{item.label}
					</a>
				</li>
			{/each}
		</ul>
	</div>

	<div class="sidebar-footer">
		<span class="network-badge">
			<span class="network-dot"></span>
			Mainnet
		</span>
	</div>
</nav>

<style>
	.sidebar {
		position: fixed;
		top: 0;
		left: 0;
		width: var(--sidebar-width);
		height: 100vh;
		background: var(--color-sidebar-bg);
		border-right: 1px solid var(--color-sidebar-border);
		display: flex;
		flex-direction: column;
		z-index: 100;
		overflow-y: auto;
	}

	.sidebar-brand {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 1.25rem 1rem;
		border-bottom: 1px solid var(--color-sidebar-border);
	}

	.brand-icon {
		width: 28px;
		height: 28px;
		background: var(--color-accent);
		color: white;
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 600;
		font-size: 0.875rem;
	}

	.brand-text {
		font-weight: 600;
		font-size: 1rem;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
	}

	.sidebar-section {
		padding: 0.5rem 0;
	}

	.sidebar-section + .sidebar-section {
		margin-top: auto;
		border-top: 1px solid var(--color-sidebar-border);
	}

	.sidebar-label {
		padding: 0.5rem 1rem 0.25rem;
		font-size: 0.6875rem;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}

	.sidebar-nav {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.sidebar-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.5rem 1rem;
		margin: 0 0.5rem;
		border-radius: 6px;
		color: var(--color-text-secondary);
		text-decoration: none;
		font-size: 0.8125rem;
		font-weight: 500;
		transition: all 150ms ease;
		cursor: pointer;
	}

	.sidebar-item:hover {
		background: var(--color-sidebar-hover);
		color: var(--color-text-primary);
	}

	.sidebar-item.active {
		background: var(--color-sidebar-active);
		color: var(--color-text-primary);
	}

	.icon {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}

	.sidebar-footer {
		padding: 1rem;
		border-top: 1px solid var(--color-sidebar-border);
	}

	.network-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		padding: 0.25rem 0.625rem;
		border-radius: 9999px;
		font-size: 0.6875rem;
		font-weight: 500;
		background: var(--color-success-muted);
		color: var(--color-success);
	}

	.network-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--color-success);
	}
</style>
