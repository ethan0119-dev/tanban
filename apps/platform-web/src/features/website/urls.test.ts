import { describe, expect, it } from 'vitest';
import { isValidWebsiteImageURL, websiteImagePreviewURL } from './urls';

describe('website image URLs', () => {
  it('accepts bundled website assets and resolves them against the official site', () => {
    expect(isValidWebsiteImageURL('/website/scan-ordering.png')).toBe(true);
    expect(websiteImagePreviewURL('/website/scan-ordering.png')).toBe('https://tanban.com.cn/website/scan-ordering.png');
  });

  it('accepts HTTPS uploads and rejects unsafe relative paths', () => {
    expect(isValidWebsiteImageURL('https://api.tanban.com.cn/api/v1/public/media/website/example.png')).toBe(true);
    expect(isValidWebsiteImageURL('/website/../secret.png')).toBe(false);
    expect(isValidWebsiteImageURL('javascript:alert(1)')).toBe(false);
  });
});
