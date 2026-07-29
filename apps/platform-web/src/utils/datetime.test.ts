import { describe, expect, it } from 'vitest';
import { formatBeijingDate, formatBeijingDateTime } from './datetime';

describe('Beijing datetime contract', () => {
  it('does not depend on the browser timezone', () => {
    expect(formatBeijingDateTime('2026-07-21 14:00:00')).toBe('2026-07-21 14:00:00');
    expect(formatBeijingDateTime('2026-07-21T06:00:00Z')).toBe('2026-07-21 14:00:00');
    expect(formatBeijingDateTime('2026-07-21T14:00:00+08:00')).toBe('2026-07-21 14:00:00');
    expect(formatBeijingDate('2026-07-21T16:30:00Z')).toBe('2026-07-22');
  });
});
