import { beforeEach, describe, expect, it, vi } from 'vitest';
import { parseOrderingEntry } from '../miniprogram/utils/store-route';
import {
  normalizeTableCodeResolution,
  readTableOrderingContext,
  saveTableOrderingContext,
  tableOrderFields,
} from '../miniprogram/utils/table-context';
import { normalizeFastFoodResolution, readFastFoodContext, saveFastFoodContext } from '../miniprogram/utils/fast-food-context';
import { env } from '../miniprogram/config/env';

describe('miniapp ordering entry routes', () => {
  let storage: Map<string, unknown>;

  beforeEach(() => {
    storage = new Map();
    vi.stubGlobal('wx', {
      getStorageSync: (key: string) => storage.get(key),
      setStorageSync: (key: string, value: unknown) => storage.set(key, value),
      removeStorageSync: (key: string) => storage.delete(key),
    });
  });

  it('opens the configured decorated store when launched without a store code', () => {
    expect(env.defaultStoreCode).toBe('manong-coffee-gulou');
  });

  it('keeps ordinary store routes outside dine-in mode', () => {
    expect(parseOrderingEntry({ query: { storeCode: 'manong-coffee' }, scene: 1011 })).toEqual({
      kind: 'STORE',
      storeCode: 'manong-coffee',
    });
    expect(parseOrderingEntry({ query: { scene: 'manong-coffee' } })).toEqual({
      kind: 'STORE',
      storeCode: 'manong-coffee',
    });
    expect(parseOrderingEntry({ scene: 1011 })).toEqual({ kind: 'NONE' });
  });

  it('parses table codes from direct query, encoded custom scene, and legacy bare token', () => {
    const token = '0123456789abcdef0123456789ab';
    expect(parseOrderingEntry({ query: { table_code: token } })).toEqual({ kind: 'TABLE', publicScene: token });
    expect(parseOrderingEntry({ query: { scene: encodeURIComponent(`tc=${token}`) } })).toEqual({ kind: 'TABLE', publicScene: token });
    expect(parseOrderingEntry({ query: { scene: token } })).toEqual({ kind: 'TABLE', publicScene: token });
  });

  it('fails closed for malformed and conflicting route parameters', () => {
    expect(parseOrderingEntry({ query: { tableCode: '../bad' } }).kind).toBe('INVALID');
    expect(parseOrderingEntry({ query: { storeCode: 'shop-a', scene: 'store=shop-b&tc=0123456789abcdef0123456789ab' } }).kind).toBe('INVALID');
    expect(parseOrderingEntry({ query: { scene: 'tc=short' } }).kind).toBe('INVALID');
  });

  it('parses and normalizes fast-food pickup plate routes without confusing them with table routes', () => {
    const publicId = '0123456789abcdef0123456789ab';
    expect(parseOrderingEntry({ query: { scene: `fp=${publicId}` } })).toEqual({ kind: 'FAST_FOOD', publicId });
    expect(parseOrderingEntry({ query: { fast_food_plate: publicId, storeCode: 'manong-coffee' } })).toEqual({ kind: 'FAST_FOOD', publicId, expectedStoreCode: 'manong-coffee' });
    expect(parseOrderingEntry({ query: { fp: 'short' } }).kind).toBe('INVALID');

    const now = 1_800_000_000_000;
    const context = normalizeFastFoodResolution(publicId, { publicId, storeCode: 'manong-coffee', storeName: '码农咖啡', plateCode: 'K08', plateName: '取餐架 K08', status: 'ACTIVE' }, now);
    vi.spyOn(Date, 'now').mockReturnValue(now);
    saveFastFoodContext(context);
    expect(readFastFoodContext(now)).toMatchObject({ storeCode: 'manong-coffee', plateCode: 'K08' });
  });

  it('normalizes, stores, expires, and emits only explicit dine-in order fields', () => {
    const now = 1_800_000_000_000;
    const context = normalizeTableCodeResolution('0123456789abcdef0123456789ab', {
      store: { id: 1, code: 'manong-coffee', name: '码农咖啡' },
      table: { publicId: 'table-public-b02', name: 'B02桌', tableCode: 'B02', areaName: '院内' },
    }, now);
    vi.spyOn(Date, 'now').mockReturnValue(now);
    saveTableOrderingContext(context);

    expect(readTableOrderingContext(now)).toMatchObject({ storeCode: 'manong-coffee', tableName: 'B02桌' });
    expect(tableOrderFields(context)).toEqual({ order_scene: 'DINE_IN', table_public_id: 'table-public-b02' });
    expect(tableOrderFields(null)).toEqual({});
    expect(readTableOrderingContext(context.validUntil + 1)).toBeNull();
  });
});
