const OFFICIAL_WEBSITE_ORIGIN = 'https://tanban.com.cn';

export function isValidWebsiteImageURL(value: string): boolean {
  const candidate = value.trim();
  if (!candidate) return true;
  if (candidate.startsWith('/website/') && !candidate.includes('..') && !/[?#\\]/.test(candidate)) return true;
  try {
    const parsed = new URL(candidate);
    return parsed.protocol === 'https:' ||
      (parsed.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(parsed.hostname));
  } catch {
    return false;
  }
}

export function websiteImagePreviewURL(value: string): string {
  const candidate = value.trim();
  return candidate.startsWith('/website/') ? `${OFFICIAL_WEBSITE_ORIGIN}${candidate}` : candidate;
}
