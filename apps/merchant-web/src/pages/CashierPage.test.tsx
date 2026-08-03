import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { App as AntApp } from 'antd';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CashierPage, cashierOrderStatusText } from './CashierPage';

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({
    user: { id: 1, name: '测试店主', storeName: '测试门店' },
    logout: vi.fn(),
  }),
}));

function renderCashier(previewCashierEnabled = true) {
  return render(
    <MemoryRouter>
      <AntApp>
        <CashierPage previewMode previewCashierEnabled={previewCashierEnabled} />
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

  it('shows a prominent read-only preview when the cashier is not enabled', () => {
    const view = renderCashier(false);

    expect(screen.getByText('收银台未开通，请联系管理员开通')).toBeTruthy();
    expect(screen.getByText(/以下为收银台功能预览/)).toBeTruthy();
    expect(view.container.querySelector('.cashier-shell.is-unavailable')).toBeTruthy();
    expect(view.container.querySelector('.cashier-stage')?.getAttribute('aria-disabled')).toBe('true');
  });

  it('opens the diner count editor from the persistent action dock', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /修改人数/ }));

    expect(await screen.findByRole('button', { name: '保存人数' })).toBeTruthy();
  });

  it('opens the settlement method selector', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /结账 ¥132/ }));

    expect(await screen.findByRole('button', { name: /现金收款/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /微信付款码/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /系统外支付/ })).toBeTruthy();
    expect(screen.getByText('本次应收')).toBeTruthy();
  });

  it.each([
    {
      method: /现金收款/,
      title: '确认现金已收妥并结账？',
      warning: '请先清点并收妥现金',
      confirm: '确认现金已收妥',
    },
    {
      method: /系统外支付/,
      title: '确认系统外款项已到账并结账？',
      warning: '请先在对应收款渠道确认到账',
      confirm: '确认外部款项已到账',
    },
  ])('requires a second confirmation for $title', async ({ method, title, warning, confirm }) => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /结账 ¥132/ }));
    fireEvent.click(await screen.findByRole('button', { name: method }));

    expect((await screen.findAllByText(title)).length).toBeGreaterThan(0);
    expect(screen.getByText(warning)).toBeTruthy();
    expect(screen.getByTestId('offline-settlement-summary').textContent).toContain('双人桌（桌号 B03）');
    expect(screen.getByTestId('offline-settlement-summary').textContent).toContain('¥132.00');
    expect(screen.getByRole('button', { name: confirm })).toBeTruthy();
    expect(screen.getByRole('button', { name: '返回核对' })).toBeTruthy();
  });

  it('accepts a WeChat payment code without keeping it in the checkout UI', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /结账 ¥132/ }));
    fireEvent.click(await screen.findByRole('button', { name: /微信付款码/ }));

    const input = await screen.findByPlaceholderText('18 位微信付款码');
    fireEvent.change(input, { target: { value: '101234567890123456' } });
    fireEvent.click(screen.getByRole('button', { name: '确认收款' }));

    expect(await screen.findByText('微信支付成功')).toBeTruthy();
    expect(screen.queryByDisplayValue('101234567890123456')).toBeNull();
  });

  it('keeps rapid table switching in compact mode and focuses only in standard mode', () => {
    const view = renderCashier();
    expect(view.container.querySelector('.cashier-workspace.is-compact')).toBeTruthy();
    expect(view.container.querySelector('.cashier-workspace.is-detail-focused')).toBeNull();
    expect(view.container.querySelector('.cashier-operation')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /B06.*待结账/ }));
    expect(view.container.querySelector('.cashier-workspace.is-detail-focused')).toBeNull();
    expect(view.container.querySelector('.cashier-operation')).toBeTruthy();

    fireEvent.click(screen.getByText('标准模式'));
    expect(view.container.querySelector('.cashier-workspace.is-standard')).toBeTruthy();
    expect(view.container.querySelector('.cashier-operation')).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /B03.*待结账/ }));
    expect(view.container.querySelector('.cashier-workspace.is-detail-focused')).toBeTruthy();
    expect(screen.getByText('订单与桌台操作')).toBeTruthy();
    expect(screen.getByText('结账与支付')).toBeTruthy();
    expect(screen.getByText('支付记录')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /返回桌台列表/ }));
    expect(view.container.querySelector('.cashier-workspace.is-detail-focused')).toBeNull();
    expect(screen.getByRole('button', { name: /B03.*待结账/ })).toBeTruthy();
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

  it('returns dishes directly with a preset reason instead of an approval form', async () => {
    renderCashier();
    fireEvent.click(screen.getByRole('button', { name: /minus 退菜$/ }));

    expect(await screen.findByText('确认后立即生效')).toBeTruthy();
    expect(screen.getByRole('button', { name: '确认退菜并更新账单' })).toBeTruthy();
    expect(screen.queryByText(/提交审批|审核退菜/)).toBeNull();

    const reasonPlaceholder = screen.getByText('选择退菜原因');
    fireEvent.mouseDown(reasonPlaceholder.closest('.ant-select-selector')!);
    expect(await screen.findByRole('option', { name: '顾客点错' })).toBeTruthy();
    expect(screen.getByRole('option', { name: '顾客临时取消' })).toBeTruthy();
  });

  it('filters the table board from operational alerts and never invents overdue counts', () => {
    renderCashier();
    const namedTable = screen.getByRole('button', { name: /B03.*双人桌.*待结账/ });
    expect(namedTable.querySelector('strong')?.textContent).toBe('B03');
    expect(namedTable.querySelector('.cashier-table-name')?.textContent).toBe('双人桌');
    expect(screen.getByRole('button', { name: /1单超时待取/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /A02.*待清台/ })).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /3桌待结账/ }));
    expect(screen.getByRole('button', { name: /B03/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /B06/ })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /A01/ })).toBeNull();
  });

  it('shows only the creation entry for the active scene and never offers additions on takeout orders', () => {
    renderCashier();
    expect(screen.getAllByRole('button', { name: /新开桌/ })).toHaveLength(1);
    expect(screen.queryByRole('button', { name: /新建带走单/ })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: /带走点单/ }));

    expect(screen.getAllByRole('button', { name: /新建带走单/ })).toHaveLength(1);
    expect(screen.queryByRole('button', { name: /新开桌/ })).toBeNull();
    expect(screen.getByText('请取餐')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /038/ }));
    expect(screen.queryByRole('button', { name: /点单开台/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /加菜/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /修改人数/ })).toBeNull();
    expect(screen.getByRole('button', { name: /打印客户联/ })).toBeTruthy();
    expect(cashierOrderStatusText({
      status: 'COMPLETED',
      paymentStatus: 'PAID',
      settlementMode: 'PAY_BEFORE',
    })).toBe('已完成');
  });
});
