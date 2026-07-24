import { describe, expect, it } from 'vitest';
import type { Product } from '../types';
import { normalizeProduct, productPayload } from './ProductsPage';

const baseProduct: Product = {
  id: 9,
  name: '经典美式',
  categoryId: 2,
  price: 9.9,
  stock: 20,
  enabled: true,
  recommended: true,
  skus: [{ id: 4, name: '默认规格', price: 9.9, stock: 20 }],
  images: [
    { id: 11, mediaAssetId: 21, url: 'https://cdn.test/main.png', isPrimary: true, sortOrder: 0 },
    { id: 12, mediaAssetId: 22, url: 'https://cdn.test/detail.png', isPrimary: false, sortOrder: 1 },
  ],
};

describe('product media mapping', () => {
  it('normalizes primary image and sales metadata from API snake case', () => {
    const product = normalizeProduct({
      ...baseProduct,
      image: undefined,
      salesCount: undefined,
      image_url: 'https://cdn.test/legacy.png',
      sales_count: 18,
      images: [{ id: 3, media_asset_id: 7, url: 'https://cdn.test/api.png', is_primary: true, sort_order: 0 }],
    } as unknown as Product);
    expect(product.image).toBe('https://cdn.test/api.png');
    expect(product.images?.[0]).toMatchObject({ mediaAssetId: 7, isPrimary: true });
    expect(product.salesCount).toBe(18);
  });

  it('writes the strict product image input without response-only image ids', () => {
    const payload = productPayload(baseProduct);
    expect(payload.image_url).toBe('https://cdn.test/main.png');
    expect(payload.recommended).toBe(true);
    expect(payload.images).toHaveLength(2);
    expect(payload.images[0]).toEqual(expect.objectContaining({ media_asset_id: 21, is_primary: true, sort_order: 0 }));
    expect(payload.images[0]).not.toHaveProperty('id');
  });

  it('keeps an implicit default sku when the product has no visible specifications', () => {
    const payload = productPayload({
      ...baseProduct,
      skus: [],
      baseSkuId: 4,
      baseExpectedStock: 20,
      basePrice: 9.9,
      baseStock: 20,
    } as unknown as Parameters<typeof productPayload>[0]);
    expect(payload.skus).toEqual([expect.objectContaining({
      id: 4,
      name: '默认规格',
      price_cents: 990,
      stock: 20,
      expected_stock: 20,
    })]);
  });
});
