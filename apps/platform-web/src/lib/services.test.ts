import { afterEach, describe, expect, it, vi } from 'vitest';
import { storeService } from './services';

function jsonResponse(data: unknown): Response {
  return new Response(JSON.stringify({ data }), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

describe('storeService.update', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('loads store detail and preserves decoration fields on a partial update', async () => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        id: 12,
        tenant_id: 7,
        code: 'COFFEE',
        name: '码农咖啡',
        logo_url: 'https://img.example/logo.png',
        banner_url: 'https://img.example/banner.png',
        notice: '今晚见',
        status: 'ACTIVE',
      }))
      .mockResolvedValueOnce(jsonResponse({ id: 12, tenant_id: 7, name: '码农咖啡', status: 'DISABLED' }));
    vi.stubGlobal('fetch', fetchMock);

    await storeService.update('7', '12', { status: 'disabled', logoUrl: undefined });

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0][0]).toContain('/platform/tenants/7/stores/12/');
    const request = fetchMock.mock.calls[1][1] as RequestInit;
    expect(request.method).toBe('PUT');
    expect(JSON.parse(String(request.body))).toMatchObject({
      logo_url: 'https://img.example/logo.png',
      banner_url: 'https://img.example/banner.png',
      notice: '今晚见',
      status: 'DISABLED',
    });
  });
});
