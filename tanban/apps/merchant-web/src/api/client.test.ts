import { describe, expect, it } from 'vitest';
import { ApiError, toListResult, unwrapEnvelope } from './client';

describe('API envelope compatibility', () => {
  it('unwraps a standard data envelope', () => {
    expect(unwrapEnvelope({ data: { id: 1 }, meta: { total: 1 } })).toEqual({ id: 1 });
  });

  it('preserves a raw payload', () => {
    expect(unwrapEnvelope([{ id: 1 }])).toEqual([{ id: 1 }]);
  });

  it('throws an API error from the envelope', () => {
    expect(() => unwrapEnvelope({ error: { message: '未授权', code: 'UNAUTHORIZED' } }))
      .toThrow(ApiError);
  });

  it('normalizes common list shapes', () => {
    expect(toListResult<{ id: number }>({ data: { records: [{ id: 1 }], total: 3 } }))
      .toEqual({ items: [{ id: 1 }], meta: { total: 3 } });
  });
});
