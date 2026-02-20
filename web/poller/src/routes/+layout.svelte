<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ModeWatcher } from 'mode-watcher';
	import { isAuthenticated, isLoading, checkSession } from '$lib/stores/auth';
	import { getHealth } from '$lib/utils/api';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
	let network = $state('');

	// Check session on mount
	onMount(async () => {
		await checkSession();

		// Fetch network info
		try {
			const resp = await getHealth();
			network = resp.data?.network ?? '';
		} catch {
			network = '';
		}
	});

	// Auth gating: redirect to /login if not authenticated (and not already on /login)
	$effect(() => {
		if (!$isLoading && !$isAuthenticated && page.url.pathname !== '/login') {
			goto('/login');
		}
	});

	let isLoginPage = $derived(page.url.pathname === '/login');
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<ModeWatcher defaultMode="dark" />

{#if $isLoading}
	<div class="loading-screen">
		<div class="loading-spinner"></div>
	</div>
{:else if isLoginPage}
	{@render children()}
{:else if $isAuthenticated}
	<div class="app-layout">
		<Sidebar {network} />
		<main class="main-content">
			{@render children()}
		</main>
	</div>
{/if}

<style>
	.app-layout {
		display: flex;
		min-height: 100vh;
	}

	.main-content {
		flex: 1;
		margin-left: var(--sidebar-width);
		background: var(--color-bg-primary);
		min-height: 100vh;
	}

	.loading-screen {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: var(--color-bg-primary);
	}

	.loading-spinner {
		width: 32px;
		height: 32px;
		border: 3px solid var(--color-border-subtle);
		border-top-color: var(--color-accent-default);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
