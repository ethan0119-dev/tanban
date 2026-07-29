import { fireEvent, render } from '@testing-library/react';
import { Form } from 'antd';
import { describe, expect, it } from 'vitest';
import { NoSpecificationProductFields } from './ProductsPage';

describe('no-specification product fields', () => {
  it('keeps price and stock independent while the operator types', () => {
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
    const view = render(
      <Form initialValues={{ skus: [], basePrice: 9.9, baseStock: 20 }}>
        <NoSpecificationProductFields />
      </Form>,
    );
    const price = view.getByLabelText('售价') as HTMLInputElement;
    const stock = view.getByLabelText('库存') as HTMLInputElement;

    fireEvent.change(price, { target: { value: '19.8' } });
    fireEvent.change(stock, { target: { value: '30' } });
    expect(Number(price.value)).toBe(19.8);

    fireEvent.change(price, { target: { value: '22.5' } });
    expect(Number(stock.value)).toBe(30);
  });
});
