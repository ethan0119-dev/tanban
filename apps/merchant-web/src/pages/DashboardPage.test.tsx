import '@testing-library/jest-dom/vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { DashboardPage } from './DashboardPage';
import { api } from '../api/client';

const nativeGetComputedStyle = window.getComputedStyle.bind(window);

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({
    user: {
      id: 1,
      name: '店主',
      roles: ['MERCHANT_OWNER'],
      capabilities: ['VIEW_FINANCIALS'],
    },
  }),
}));

vi.mock('../api/client', () => ({
  api: {
    get: vi.fn(),
  },
  errorMessage: (error: unknown) => error instanceof Error ? error.message : '请求失败',
}));

describe('DashboardPage analytics empty states', () => {
  beforeAll(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    vi.spyOn(window, 'getComputedStyle').mockImplementation((element) => nativeGetComputedStyle(element));
  });

  beforeEach(() => {
    vi.mocked(api.get).mockResolvedValue({
      today_revenue_cents: 0,
      today_orders: 0,
      active_orders: 0,
      monthlyTrend: [],
      todayOrderTypes: [],
      todayHourly: [],
      recentOrders: [],
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  afterAll(() => {
    vi.restoreAllMocks();
  });

  it('keeps monthly and today distribution cards visible when analytics arrays are empty', async () => {
    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>,
    );

    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/merchant/dashboard'));
    expect(await screen.findByText('本月营业曲线')).toBeInTheDocument();
    expect(screen.getByText('暂无本月数据')).toBeInTheDocument();
    expect(screen.getByText('今日订单类型分布')).toBeInTheDocument();
    expect(screen.getByText('今日暂无订单类型数据')).toBeInTheDocument();
    expect(screen.getByText('今日时段订单分布')).toBeInTheDocument();
    expect(screen.getByText('今日暂无时段订单数据')).toBeInTheDocument();
  });
});
