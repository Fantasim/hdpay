import { getAddresses, type AddressListParams } from '$lib/utils/api';
import type { AddressWithBalance, APIMeta, Chain } from '$lib/types';

interface AddressState {
	addresses: AddressWithBalance[];
	meta: APIMeta | null;
	loading: boolean;
	error: string | null;
	chain: Chain;
	page: number;
	pageSize: number;
	hasBalance: boolean;
	token: string;
}

const DEFAULT_PAGE_SIZE = 100;

function createAddressStore() {
	let state = $state<AddressState>({
		addresses: [],
		meta: null,
		loading: false,
		error: null,
		chain: 'BTC',
		page: 1,
		pageSize: DEFAULT_PAGE_SIZE,
		hasBalance: false,
		token: ''
	});

	async function fetchAddresses(): Promise<void> {
		state.loading = true;
		state.error = null;

		try {
			const params: AddressListParams = {
				page: state.page,
				pageSize: state.pageSize
			};
			if (state.hasBalance) params.hasBalance = true;
			if (state.token) params.token = state.token;

			const response = await getAddresses(state.chain, params);
			state.addresses = response.data ?? [];
			state.meta = response.meta ?? null;
		} catch (err) {
			state.error = err instanceof Error ? err.message : 'Failed to fetch addresses';
			state.addresses = [];
			state.meta = null;
		} finally {
			state.loading = false;
		}
	}

	function setChain(chain: Chain): void {
		state.chain = chain;
		state.page = 1;
		fetchAddresses();
	}

	function setPage(page: number): void {
		state.page = page;
		fetchAddresses();
	}

	function setFilter(filter: { hasBalance?: boolean; token?: string }): void {
		if (filter.hasBalance !== undefined) state.hasBalance = filter.hasBalance;
		if (filter.token !== undefined) state.token = filter.token;
		state.page = 1;
		fetchAddresses();
	}

	return {
		get state() {
			return state;
		},
		fetchAddresses,
		setChain,
		setPage,
		setFilter
	};
}

export const addressStore = createAddressStore();
