export type StoreRouteQuery = Record<string, string | undefined>;

const STORE_CODE_PATTERN = /^[a-zA-Z0-9_-]{1,64}$/;

function decode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

function validStoreCode(value: string | undefined): string {
  if (!value) return "";
  const decoded = decode(value);
  return STORE_CODE_PATTERN.test(decoded) ? decoded : "";
}

function storeCodeFromScene(rawScene: string | undefined): string {
  if (!rawScene) return "";
  const scene = decode(rawScene);
  if (!scene.includes("=")) return validStoreCode(scene);

  const fields = scene.split("&").reduce<Record<string, string>>((result, field) => {
    const separator = field.indexOf("=");
    if (separator < 0) return result;
    const key = decode(field.slice(0, separator));
    result[key] = decode(field.slice(separator + 1));
    return result;
  }, {});
  return validStoreCode(fields.storeCode) || validStoreCode(fields.store) || validStoreCode(fields.s);
}

/** Returns an explicitly routed store, or an empty string when this route has none. */
export function routedStoreCode(query: StoreRouteQuery): string {
  return validStoreCode(query.storeCode)
    || validStoreCode(query.store)
    || validStoreCode(query.s)
    || storeCodeFromScene(query.scene);
}
