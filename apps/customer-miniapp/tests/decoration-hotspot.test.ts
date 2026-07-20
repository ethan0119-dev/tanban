import { describe, expect, it } from 'vitest';
import { normalizeDecoration } from '../miniprogram/utils/decoration';

describe('miniapp hotspot decoration', () => {
  it('keeps valid hotspot actions and percentage geometry', () => {
    const config = normalizeDecoration({
      home: { modules: [{
        id: 'home-image',
        type: 'HOTSPOT_IMAGE',
        enabled: true,
        sortOrder: 10,
        config: {
          imageUrl: 'https://cdn.example.com/home.webp',
          alt: '首页导航',
          hotspots: [{ id: 'orders', x: 10, y: 15, width: 35, height: 20, label: '查看订单', action: { type: 'OPEN_ORDERS' } }],
        },
      }] },
    });

    expect(config.home.modules[0]).toMatchObject({
      type: 'HOTSPOT_IMAGE',
      config: { hotspots: [{ id: 'orders', x: 10, y: 15, width: 35, height: 20, label: '查看订单', action: { type: 'OPEN_ORDERS' } }] },
    });
  });

  it('clamps overflow and rejects arbitrary action types', () => {
    const config = normalizeDecoration({
      home: { modules: [{
        id: 'home-image',
        type: 'HOTSPOT_IMAGE',
        enabled: true,
        sortOrder: 10,
        config: { hotspots: [{ id: 'bad', x: 96, y: 99, width: 80, height: 30, label: '不安全', action: { type: 'OPEN_URL', url: 'https://invalid.example' } }] },
      }] },
    });

    expect(config.home.modules[0].config.hotspots?.[0]).toMatchObject({ x: 96, y: 99, width: 4, height: 1, action: { type: 'NONE' } });
  });
});
