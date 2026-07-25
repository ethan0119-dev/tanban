import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { App as AntApp } from 'antd';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CashierPage } from './CashierPage';

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: '测试店主', storeName: '测试门店' },
    logout: vi.fn(),
  }),
}));

function renderCashier() {
  return render(
    <MemoryRouter>
      <AntApp>
        <CashierPage previewMode />
      </AntApp>
    </MemoryRouter>,
  );
}

describe('cashier operations', () => {
  beforeEach(() => {
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

  afterEach(() => cleanup());

  it('opens the diner count editor from the persistent action dock', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /修改人数/ }));

    expect(await screen.findByRole('button', { name: '保存人数' })).toBeTruthy();
  });

  it('opens the settlement method selector', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /结账 ¥132/ }));

    expect(await screen.findByRole('button', { name: /现金收款/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /系统外支付/ })).toBeTruthy();
  });

  it('opens transfer and handover workflows', async () => {
    const view = renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /转台$/ }));
    expect(await screen.findByRole('button', { name: '确认转台' })).toBeTruthy();

    view.unmount();
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: '交接班' }));
    expect(await screen.findByText('确认交接并退出收银台')).toBeTruthy();
    expect(screen.getByPlaceholderText(/交接备注/)).toBeTruthy();
  });

  it('filters the table board from operational alerts and never invents overdue counts', () => {
    renderCashier();
    expect(screen.getByRole('button', { name: /1单超时待取/ })).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /3桌待结账/ }));
    expect(screen.getByRole('button', { name: /B03/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /B06/ })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /A01/ })).toBeNull();
  });
});
