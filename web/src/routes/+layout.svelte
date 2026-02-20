<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/layout/Sidebar.svelte';
	import { API_BASE } from '$lib/constants';

	let { children } = $props();
	let network: string = $state('');

	onMount(async () => {
		try {
			const resp = await fetch(`${API_BASE}/health`);
			const data = await resp.json();
			network = data.network || '';
		} catch {
			network = '';
		}
	});
</script>

<svelte:head>
	<title>HDPay{network === 'testnet' ? ' (Testnet)' : ''}</title>
</svelte:head>

<div class="app-layout">
	<Sidebar />
	<main class="main-content">
		{#if network === 'testnet'}
			<div class="testnet-banner">TESTNET MODE — Balances are not real</div>
		{/if}
		{@render children()}
	</main>
</div>

<style>
	.app-layout {
		display: flex;
		min-height: 100vh;
	}

	.main-content {
		flex: 1;
		margin-left: var(--sidebar-width);
		padding: 2rem;
		max-width: calc(100vw - var(--sidebar-width));
	}

	.testnet-banner {
		background: var(--color-warning-muted, rgba(234, 179, 8, 0.15));
		color: var(--color-warning, #eab308);
		text-align: center;
		padding: 0.375rem 1rem;
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.05em;
		border-radius: 6px;
		margin-bottom: 1rem;
	}
</style>
