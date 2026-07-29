import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { StaffPage } from './StaffPage';

const apiMock = vi.hoisted(() => ({
  getList: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}));

vi.mock('../api/client', () => ({
  api: apiMock,
  errorMessage: (error: unknown) => error instanceof Error ? error.message : '请求失败',
}));

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 1,
      name: '测试老板',
      roles: ['MERCHANT_OWNER'],
      capabilities: [],
    },
  }),
}));

describe('staff role permission summary', () => {
  beforeEach(() => {
    apiMock.getList.mockReset();
    apiMock.getList.mockResolvedValue({ items: [], meta: { total: 0 } });
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: () => ({
        matches: false,
        addListener: () => undefined,
        removeListener: () => undefined,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
        dispatchEvent: () => false,
      }),
    });
    class ResizeObserverMock {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
    Object.defineProperty(window, 'ResizeObserver', { writable: true, value: ResizeObserverMock });
    const getComputedStyle = window.getComputedStyle.bind(window);
    Object.defineProperty(window, 'getComputedStyle', {
      writable: true,
      value: (element: Element) => getComputedStyle(element),
    });
  });

  it('keeps permission tags out of role cards until the info icon is hovered', async () => {
    render(<StaffPage />);
    await waitFor(() => expect(apiMock.getList).toHaveBeenCalled());

    expect(screen.queryByText('经营总览')).toBeNull();
    const permissionButton = screen.getByRole('button', { name: '查看老板权限' });
    fireEvent.mouseEnter(permissionButton);

    expect(await screen.findByText('老板权限')).toBeTruthy();
    expect(screen.getByText('经营总览')).toBeTruthy();
    expect(screen.getByText('商品与库存')).toBeTruthy();
  });
});
