import { describe, expect, it } from 'vitest';
import { storeShareMessage, storeTimelineMessage } from '../miniprogram/utils/share';

describe('store sharing', () => {
  it('shares the active store without leaking table or customer context', () => {
    const store = { code: 'manong-coffee-gulou', name: '码农咖啡鼓楼店', logoUrl: 'https://cdn.example/logo.png' } as never;
    expect(storeShareMessage(store, 'fallback')).toEqual({
      title: '码农咖啡鼓楼店｜微信点单',
      path: '/pages/home/index?storeCode=manong-coffee-gulou',
      imageUrl: 'https://cdn.example/logo.png',
    });
    expect(storeTimelineMessage(store, 'fallback').query).toBe('storeCode=manong-coffee-gulou');
  });

  it('uses the configured default store on a parameterless launch', () => {
    expect(storeShareMessage(undefined, 'manong-coffee-gulou').path).toBe('/pages/home/index?storeCode=manong-coffee-gulou');
  });
});
