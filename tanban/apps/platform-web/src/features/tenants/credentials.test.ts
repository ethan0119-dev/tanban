import { describe, expect, it } from 'vitest';
import { generateInitialPassword, generatedOwnerUsername, usernameFromStoreName } from './credentials';

describe('merchant owner credentials', () => {
  it('generates a lowercase pinyin account from the store name', () => {
    expect(usernameFromStoreName('码农咖啡 主门店')).toBe('manongkafeizhumendian');
  });

  it('supports phone and pinyin username modes', () => {
    expect(generatedOwnerUsername('PHONE', '码农咖啡', '186 0229 6557')).toBe('18602296557');
    expect(generatedOwnerUsername('PINYIN', '码农咖啡', '')).toBe('manongkafei');
  });

  it('generates a password with mixed character classes', () => {
    const password = generateInitialPassword();
    expect(password).toHaveLength(14);
    expect(password).toMatch(/[a-z]/);
    expect(password).toMatch(/[A-Z]/);
    expect(password).toMatch(/[0-9]/);
    expect(password).toMatch(/[!@#$%]/);
  });
});
