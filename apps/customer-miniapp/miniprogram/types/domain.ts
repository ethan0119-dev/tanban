export interface StoreTheme {
  primaryColor?: string;
  bannerUrl?: string;
  announcement?: string;
}

export interface Store {
  id: number;
  code: string;
  name: string;
  logoUrl?: string;
  address?: string;
  businessStatus: "OPEN" | "CLOSED";
  theme?: StoreTheme;
}

export interface Category {
  id: number;
  name: string;
  sortOrder: number;
}

export interface Sku {
  id: number;
  name: string;
  price: number;
  stock: number;
  soldOut: boolean;
}

export interface Product {
  id: number;
  categoryId: number;
  name: string;
  description?: string;
  imageUrl?: string;
  price: number;
  stock: number;
  soldOut: boolean;
  skus?: Sku[];
}

export interface CartItem {
  productId: number;
  skuId?: number;
  name: string;
  skuName?: string;
  price: number;
  quantity: number;
  lineKey?: string;
}

export interface Order {
  id: number;
  orderNo: string;
  pickupCode?: string;
  status: string;
  paymentStatus: string;
  amount: number;
  createdAt: string;
  items?: Array<CartItem & { amount: number }>;
}
