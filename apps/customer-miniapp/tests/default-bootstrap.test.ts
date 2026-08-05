import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

describe('parameterless miniapp bootstrap', () => {
  it('loads the platform-selected default before resolving a cold-start route', () => {
    const source = readFileSync(new URL('../miniprogram/app.ts', import.meta.url), 'utf8');
    expect(source).toContain('/public/miniapp/bootstrap?channelKey=');
    expect(source).toContain('this.loadDefaultStoreCode().then(() => this.prepareOrderingEntry(options, true))');
    expect(source).toContain('this.globalData.storeCode = storeCode');
  });
});
