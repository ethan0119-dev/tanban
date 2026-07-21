import { describe, expect, it } from 'vitest';
import dayjs from 'dayjs';
import { beijingPickerValue, businessHoursRange, dateOnly, dateTime, initials, percentChange, pickupCode, toBeijingRFC3339, yuan } from './format';

describe('format helpers', () => {
  it('formats amounts safely', () => {
    expect(yuan(12)).toBe('¥12.00');
    expect(yuan(undefined)).toBe('¥0.00');
  });

  it('calculates percent change', () => {
    expect(percentChange(120, 100)).toBe(20);
    expect(percentChange(10, 0)).toBeNull();
  });

  it('returns a short avatar initial', () => {
    expect(initials('码农咖啡')).toBe('码');
  });

  it('parses strict HH:mm business hours', () => {
    const range = businessHoursRange(['18:05', '02:30']);
    expect(range?.map((value) => value.format('HH:mm'))).toEqual(['18:05', '02:30']);
    expect(businessHoursRange(['25:00', '02:30'])).toBeUndefined();
    expect(businessHoursRange(['18:00'])).toBeUndefined();
  });

  it('matches the API four-digit pickup-code rule', () => {
    expect(pickupCode(7)).toBe('0007');
    expect(pickupCode(12_345)).toBe('2345');
    expect(pickupCode('invalid')).toBe('');
  });

  it('formats every visible datetime as a Beijing wall clock', () => {
    expect(dateTime('2026-07-21 14:00:00')).toBe('2026-07-21 14:00:00');
    expect(dateTime('2026-07-21T06:00:00Z')).toBe('2026-07-21 14:00:00');
    expect(dateTime('2026-07-21T14:00:00+08:00')).toBe('2026-07-21 14:00:00');
    expect(dateOnly('2026-07-21T16:30:00Z')).toBe('2026-07-22');
  });

  it('keeps picker input and payload explicitly in Beijing time', () => {
    expect(beijingPickerValue('2026-07-21T06:00:00Z')?.format('YYYY-MM-DD HH:mm:ss')).toBe('2026-07-21 14:00:00');
    expect(toBeijingRFC3339(dayjs('2026-07-21 14:00:00'))).toBe('2026-07-21T14:00:00+08:00');
  });
});
