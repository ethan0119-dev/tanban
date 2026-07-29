import { describe, expect, it } from 'vitest';
import { normalizePage } from './api';

describe('normalizePage', () => {
  it('适配标准 items + meta 响应', () => {
    const result = normalizePage<{ id: string }>(
      { items: [{ id: '1' }] },
      { page: 2, pageSize: 10, total: 21 },
    );
    expect(result.items).toEqual([{ id: '1' }]);
    expect(result.meta).toEqual({ page: 2, pageSize: 10, total: 21 });
  });

  it('适配 records 与内嵌分页响应', () => {
    const result = normalizePage<{ id: string }>({
      records: [{ id: '2' }],
      pagination: { page: 3, perPage: 5, total: 12 },
    });
    expect(result.items[0].id).toBe('2');
    expect(result.meta).toEqual({ page: 3, pageSize: 5, total: 12 });
  });

  it('数组响应使用调用方传入的分页默认值', () => {
    const result = normalizePage([{ id: '3' }], undefined, 4, 50);
    expect(result.meta).toEqual({ page: 4, pageSize: 50, total: 1 });
  });
});
