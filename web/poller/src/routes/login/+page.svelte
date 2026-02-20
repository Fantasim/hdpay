<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { login, authError, isAuthenticated } from '$lib/stores/auth';
	import { getHealth } from '$lib/utils/api';
	import { goto } from '$app/navigation';

	let username = $state('');
	let password = $state('');
	let submitting = $state(false);
	let network = $state('');

	// If already authenticated, redirect to home
	$effect(() => {
		if ($isAuthenticated) {
			goto('/');
		}
	});

	onMount(async () => {
		try {
			const resp = await getHealth();
			network = resp.data?.network ?? '';
		} catch {
			network = '';
		}
	});

	async function handleSubmit(e: Event): Promise<void> {
		e.preventDefault();
		if (submitting) return;
		submitting = true;
		await login(username, password);
		submitting = false;
	}
</script>

<svelte:head>
	<title>Poller — Sign In</title>
</svelte:head>

<div class="login-page">
	<div class="login-card">
		<!-- Brand -->
		<div class="login-brand">
			<div class="brand-icon">P</div>
			<div class="brand-name">Poller</div>
		</div>

		<!-- Subtitle -->
		<div class="login-subtitle">Crypto-to-Points Dashboard</div>

		<!-- Error Message -->
		{#if $authError}
			<div class="alert-error">
				<svg
					class="alert-icon"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
				>
					<circle cx="12" cy="12" r="10"></circle>
					<line x1="12" y1="8" x2="12" y2="12"></line>
					<line x1="12" y1="16" x2="12.01" y2="16"></line>
				</svg>
				<span>{$authError}</span>
			</div>
		{/if}

		<!-- Login Form -->
		<form class="login-form" onsubmit={handleSubmit}>
			<div class="form-group">
				<Label for="username">Username</Label>
				<Input
					id="username"
					type="text"
					placeholder="Enter your username"
					autocomplete="username"
					bind:value={username}
					required
				/>
			</div>

			<div class="form-group">
				<Label for="password">Password</Label>
				<Input
					id="password"
					type="password"
					placeholder="Enter your password"
					autocomplete="current-password"
					bind:value={password}
					required
				/>
			</div>

			<Button type="submit" class="w-full" size="lg" disabled={submitting}>
				{submitting ? 'Signing in...' : 'Sign In'}
			</Button>
		</form>

		<!-- Network Badge -->
		{#if network}
			<div class="network-badge-container">
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
</div>

<style>
	.login-page {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: linear-gradient(
			135deg,
			var(--color-bg-primary) 0%,
			var(--color-bg-secondary) 100%
		);
	}

	.login-card {
		width: 400px;
		background: var(--color-bg-surface);
		border: 1px solid var(--color-border-subtle);
		border-radius: 12px;
		padding: 2rem;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
		animation: slideUp 0.5s ease-out;
	}

	@keyframes slideUp {
		from {
			opacity: 0;
			transform: translateY(20px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.login-brand {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		margin-bottom: 2rem;
	}

	.brand-icon {
		width: 40px;
		height: 40px;
		background: var(--color-accent-default);
		border-radius: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 1rem;
		font-weight: 600;
		color: white;
	}

	.brand-name {
		font-size: 1.375rem;
		font-weight: 600;
		color: var(--color-text-primary);
		letter-spacing: -0.01em;
	}

	.login-subtitle {
		text-align: center;
		font-size: 0.8125rem;
		color: var(--color-text-muted);
		margin-top: -1.5rem;
		margin-bottom: 2rem;
	}

	.login-form {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.alert-error {
		display: flex;
		align-items: flex-start;
		gap: 0.75rem;
		padding: 1rem;
		border-radius: 8px;
		font-size: 0.8125rem;
		background: var(--color-error-muted);
		color: var(--color-error);
		border: 1px solid rgba(239, 68, 68, 0.2);
		margin-bottom: 1.25rem;
	}

	.alert-icon {
		flex-shrink: 0;
		width: 18px;
		height: 18px;
		margin-top: 1px;
	}

	.network-badge-container {
		display: flex;
		justify-content: center;
		margin-top: 2rem;
		padding-top: 1.5rem;
		border-top: 1px solid var(--color-border-subtle);
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
