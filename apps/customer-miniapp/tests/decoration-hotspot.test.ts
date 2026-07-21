import { describe, expect, it } from 'vitest';
import { decorationStyle, defaultDecoration, normalizeDecoration } from '../miniprogram/utils/decoration';
import { hasMarketingPopupVisual } from '../miniprogram/utils/marketing';

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

describe('merchant decoration chrome', () => {
  it('keeps a supported navigation icon template and falls back safely', () => {
    expect(normalizeDecoration({ navigation: { templateKey: 'warm' } }).navigation.templateKey).toBe('warm');
    expect(normalizeDecoration({ navigation: { templateKey: 'CUSTOM_HTML' } }).navigation.templateKey).toBe('classic');
  });

  it('does not render an empty marketing popup', () => {
    const base = { id: 1, name: '空弹窗', placement_code: 'HOME_POPUP', image_url: '', action_type: 'OPEN_MENU', frequency: 'EVERY_VISIT', priority: 1 } as const;
    expect(hasMarketingPopupVisual(base)).toBe(false);
    expect(hasMarketingPopupVisual({ ...base, title: '今日活动' })).toBe(true);
  });

  it('normalizes coordinated typography, surface and button choices', () => {
    const base = defaultDecoration();
    const config = normalizeDecoration({
      ...base,
      theme: { ...base.theme, fontScale: 'LARGE', surfaceStyle: 'BORDERED', buttonShape: 'PILL' },
    });
    const style = decorationStyle(config);
    expect(style).toContain('--font-title:40rpx');
    expect(style).toContain('--button-radius:999rpx');
    expect(style).toContain('--card-shadow:none');
  });
});
