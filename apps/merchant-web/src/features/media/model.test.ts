import { describe, expect, it } from 'vitest';
import { normalizeMediaAsset, normalizeMediaGroup } from './model';

describe('media model', () => {
  it('normalizes snake case asset metadata', () => {
    expect(normalizeMediaAsset({ id: 7, name: '首页图', url: 'https://cdn.test/home.png', kind: 'IMAGE', group_id: 3, group_name: '装修', size_bytes: 2048 })).toMatchObject({
      id: 7,
      name: '首页图',
      groupId: 3,
      groupName: '装修',
      sizeBytes: 2048,
      type: 'IMAGE',
    });
  });

  it('normalizes group counters', () => {
    expect(normalizeMediaGroup({ id: 3, name: '商品', sort_order: 10, asset_count: 6 })).toEqual(expect.objectContaining({ id: 3, name: '商品', sortOrder: 10, assetCount: 6 }));
  });
});
