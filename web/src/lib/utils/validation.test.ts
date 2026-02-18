import { describe, it, expect } from 'vitest';
import { validateAddress, isValidDestination } from './validation';

describe('validateAddress', () => {
	describe('BTC', () => {
		it('accepts valid bech32 mainnet address', () => {
			expect(validateAddress('BTC', 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu')).toBeNull();
		});

		it('accepts valid bech32 testnet address', () => {
			expect(validateAddress('BTC', 'tb1qtk89me2ae95dmlp3yfl4q9ynpux8mxjujuf2fr')).toBeNull();
		});

		it('accepts valid legacy P2PKH address', () => {
			expect(validateAddress('BTC', '1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa')).toBeNull();
		});

		it('accepts valid legacy P2SH address', () => {
			expect(validateAddress('BTC', '3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy')).toBeNull();
		});

		it('rejects empty address', () => {
			expect(validateAddress('BTC', '')).toBe('Destination address is required');
		});

		it('rejects invalid format', () => {
			const result = validateAddress('BTC', 'notavalidaddress');
			expect(result).not.toBeNull();
			expect(result).toContain('Invalid BTC address');
		});

		it('rejects address that is too short', () => {
			const result = validateAddress('BTC', 'bc1q');
			expect(result).not.toBeNull();
		});
	});

	describe('BSC', () => {
		it('accepts valid checksummed address', () => {
			expect(validateAddress('BSC', '0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb')).toBeNull();
		});

		it('accepts valid lowercase address', () => {
			expect(validateAddress('BSC', '0xf278cf59f82edcf871d630f28ecc8056f25c1cdb')).toBeNull();
		});

		it('rejects missing 0x prefix', () => {
			const result = validateAddress('BSC', 'F278cF59F82eDcf871d630F28EcC8056f25C1cdb');
			expect(result).not.toBeNull();
			expect(result).toContain('Invalid BSC address');
		});

		it('rejects too short address', () => {
			const result = validateAddress('BSC', '0x1234');
			expect(result).not.toBeNull();
		});

		it('rejects empty address', () => {
			expect(validateAddress('BSC', '')).toBe('Destination address is required');
		});

		it('rejects non-hex characters', () => {
			const result = validateAddress('BSC', '0xGGGGcF59F82eDcf871d630F28EcC8056f25C1cdb');
			expect(result).not.toBeNull();
		});
	});

	describe('SOL', () => {
		it('accepts valid base58 address', () => {
			expect(validateAddress('SOL', '3Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSnx')).toBeNull();
		});

		it('accepts valid 32-char address', () => {
			expect(validateAddress('SOL', '11111111111111111111111111111111')).toBeNull();
		});

		it('rejects empty address', () => {
			expect(validateAddress('SOL', '')).toBe('Destination address is required');
		});

		it('rejects invalid characters (contains 0)', () => {
			const result = validateAddress('SOL', '0Cy3YNTFywCmxoxt8n7UH6hg6dLo5uACowX3CFceaSn');
			expect(result).not.toBeNull();
			expect(result).toContain('Invalid SOL address');
		});

		it('rejects too short address', () => {
			const result = validateAddress('SOL', 'abc');
			expect(result).not.toBeNull();
		});

		it('rejects address with invalid chars (O, I, l)', () => {
			const result = validateAddress('SOL', 'OOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOO');
			expect(result).not.toBeNull();
		});
	});
});

describe('isValidDestination', () => {
	it('returns true for valid BTC address', () => {
		expect(isValidDestination('BTC', 'bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu')).toBe(true);
	});

	it('returns false for invalid BTC address', () => {
		expect(isValidDestination('BTC', 'invalid')).toBe(false);
	});

	it('returns true for valid BSC address', () => {
		expect(isValidDestination('BSC', '0xF278cF59F82eDcf871d630F28EcC8056f25C1cdb')).toBe(true);
	});

	it('returns false for empty address', () => {
		expect(isValidDestination('SOL', '')).toBe(false);
	});
});
