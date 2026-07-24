import { describe, expect, it } from 'vitest';
import { gcj02ToWgs84, roundedCoordinate, wgs84ToGcj02 } from './location';

describe('store location coordinate conversion', () => {
  it('round-trips a mainland coordinate closely enough for map picking', () => {
    const wgs = { latitude: 39.9042, longitude: 116.4074 };
    const gcj = wgs84ToGcj02(wgs);
    const restored = gcj02ToWgs84(gcj);
    expect(gcj.latitude).not.toBe(wgs.latitude);
    expect(Math.abs(restored.latitude - wgs.latitude)).toBeLessThan(0.00001);
    expect(Math.abs(restored.longitude - wgs.longitude)).toBeLessThan(0.00001);
  });

  it('does not shift coordinates outside mainland China', () => {
    expect(wgs84ToGcj02({ latitude: 35.6762, longitude: 139.6503 })).toEqual({ latitude: 35.6762, longitude: 139.6503 });
  });

  it('rounds persisted coordinates to seven decimal places', () => {
    expect(roundedCoordinate({ latitude: 39.123456789, longitude: 117.987654321 })).toEqual({
      latitude: 39.1234568,
      longitude: 117.9876543,
    });
  });
});
