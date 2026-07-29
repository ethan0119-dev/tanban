import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ProductsPage } from './ProductsPage';

const apiMock = vi.hoisted(() => ({
  getList: vi.fn(),
  getBlob: vi.fn(),
  postForm: vi.fn(),
}));

vi.mock('../api/client', () => ({
  api: apiMock,
  errorMessage: (error: unknown) => error instanceof Error ? error.message : '请求失败',
}));

describe('product menu import', () => {
  beforeEach(() => {
    apiMock.getList.mockReset();
    apiMock.getBlob.mockReset();
    apiMock.postForm.mockReset();
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

  it('previews a workbook before enabling the atomic import action', async () => {
    apiMock.postForm.mockResolvedValue({
      valid: true,
      product_count: 2,
      sku_count: 3,
      existing_category_count: 1,
      new_categories: ['烘焙'],
      existing_attribute_groups: ['温度'],
      new_attribute_groups: [],
      existing_modifier_groups: [],
      new_modifier_groups: ['加浓'],
      products: [
        {
          code: 'P001',
          name: '招牌拿铁',
          category_name: '咖啡',
          sku_count: 2,
          image_count: 1,
          attribute_groups: ['温度'],
          modifier_groups: ['加浓'],
        },
      ],
      issues: [],
    });

    render(<ProductsPage />);
    fireEvent.click(screen.getByRole('button', { name: /批量导入菜单/ }));
    expect(screen.getByText('先预检，确认后一次性导入')).toBeTruthy();

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(['xlsx'], 'menu.xlsx', {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(apiMock.postForm).toHaveBeenCalledWith(
      '/merchant/products/import/preview',
      expect.any(FormData),
    ));
    expect(await screen.findByText('预检通过，可以导入')).toBeTruthy();
    expect((screen.getByRole('button', { name: /确认导入 2 个商品/ }) as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByText('烘焙')).toBeTruthy();
  });
});
