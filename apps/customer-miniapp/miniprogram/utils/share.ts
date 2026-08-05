import type { Store } from "../types/domain";

export function storeShareMessage(store: Store | null | undefined, fallbackStoreCode: string) {
  const storeCode = String(store?.code || fallbackStoreCode || "").trim();
  const title = store?.name ? `${store.name}｜微信点单` : "摊伴｜微信点单";
  return {
    title,
    path: `/pages/home/index?storeCode=${encodeURIComponent(storeCode)}`,
    ...(store?.logoUrl ? { imageUrl: store.logoUrl } : {}),
  };
}

export function storeTimelineMessage(store: Store | null | undefined, fallbackStoreCode: string) {
  const storeCode = String(store?.code || fallbackStoreCode || "").trim();
  return {
    title: store?.name ? `${store.name}｜微信点单` : "摊伴｜微信点单",
    query: `storeCode=${encodeURIComponent(storeCode)}`,
    ...(store?.logoUrl ? { imageUrl: store.logoUrl } : {}),
  };
}
