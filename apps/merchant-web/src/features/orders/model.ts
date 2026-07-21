import type { Order, OrderType } from '../../types';

/**
 * Keep the dine-in and takeaway workboards isolated even though both scenes
 * belong to the same DINE_IN business domain in the API.
 */
export function ordersForBusinessType(items: Order[], expected: OrderType): Order[] {
  return items.filter((order) => order.orderType === expected);
}
