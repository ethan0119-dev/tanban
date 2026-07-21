import { describe, expect, it } from 'vitest';
import { merchantFeatureCopy, merchantReleaseBlockers } from './copy';

const developerVocabulary = /(\bV\d+\b|\bMock\b|scene\s*=|联调|占位|对标系统|接口待接入|服务端|客户端)/i;

describe('merchant feature availability copy', () => {
  it('uses merchant-facing wording for every registered capability', () => {
    for (const [key, copy] of Object.entries(merchantFeatureCopy)) {
      expect(copy.title, key).toBeTruthy();
      expect(copy.description, key).toBeTruthy();
      expect(`${copy.title}${copy.description}`, key).not.toMatch(developerVocabulary);
    }
  });

  it('keeps the release blocker list derived from the registry', () => {
    expect(merchantReleaseBlockers.length).toBeGreaterThan(0);
    expect(merchantReleaseBlockers).toContain('OFFICIAL_MINIAPP_CODE');
    expect(merchantReleaseBlockers).toContain('ONLINE_STORED_VALUE');
    expect(merchantReleaseBlockers).toContain('COUPON_REDEMPTION');
  });
});
