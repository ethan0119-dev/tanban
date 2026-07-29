import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

describe('miniapp channel build contract', () => {
  it('keeps only public build placeholders in source configuration', () => {
    const source = readFileSync(new URL('../miniprogram/config/env.ts', import.meta.url), 'utf8');
    expect(source).toContain('channelKey: "tanban-public"');
    expect(source).toContain('defaultStoreCode: "manong-coffee-gulou"');
    expect(source).not.toMatch(/\bappSecret\s*:/);
  });
});
