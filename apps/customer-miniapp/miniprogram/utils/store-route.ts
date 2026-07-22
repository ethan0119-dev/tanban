export type StoreRouteQuery = Record<string, string | undefined>;

const INVALID_QR_MESSAGE = "二维码无效或已过期，请重新扫描门店提供的二维码";
const WRONG_STORE_QR_MESSAGE = "该二维码不适用于当前门店，请重新扫码";

export interface OrderingEntryOptions {
  query?: StoreRouteQuery;
  /** WeChat launch scene number. It is not the custom mini-program-code payload. */
  scene?: number | string;
}

const ROUTE_QUERY_KEYS = new Set(["scene", "storeCode", "store", "s", "tableCode", "table_code", "tc", "fastFoodPlate", "fast_food_plate", "fp"]);

export type OrderingEntryRoute =
  | { kind: "NONE" }
  | { kind: "STORE"; storeCode: string }
  | { kind: "TABLE"; publicScene: string; expectedStoreCode?: string }
  | { kind: "FAST_FOOD"; publicId: string; expectedStoreCode?: string }
  | { kind: "INVALID"; message: string };

const STORE_CODE_PATTERN = /^[a-zA-Z0-9_-]{1,64}$/;
const TABLE_SCENE_PATTERN = /^[a-zA-Z0-9_-]{20,32}$/;
const TABLE_KEYS = ["tableCode", "table_code", "tc"] as const;
const FAST_FOOD_KEYS = ["fastFoodPlate", "fast_food_plate", "fp"] as const;

function decode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

/** Converts the path returned by wx.scanCode into the same route shape used on app launch. */
export function orderingEntryOptionsFromScan(pathOrResult: string): OrderingEntryOptions | null {
  const raw = String(pathOrResult || "").trim();
  if (!raw) return null;
  const queryIndex = raw.indexOf("?");
  let queryText = queryIndex >= 0 ? raw.slice(queryIndex + 1) : raw;
  const hashIndex = queryText.indexOf("#");
  if (hashIndex >= 0) queryText = queryText.slice(0, hashIndex);
  queryText = queryText.replace(/^\?/, "").trim();
  if (!queryText || (queryIndex < 0 && (/^(?:https?:\/\/|pages\/)/i.test(raw)))) return null;

  if (!queryText.includes("=")) return { query: { scene: decode(queryText) } };
  const query = queryText.split("&").reduce<StoreRouteQuery>((result, field) => {
    const separator = field.indexOf("=");
    if (separator < 0) return result;
    const key = decode(field.slice(0, separator));
    if (!ROUTE_QUERY_KEYS.has(key)) return result;
    result[key] = decode(field.slice(separator + 1));
    return result;
  }, {});
  return Object.keys(query).length ? { query } : null;
}

function validStoreCode(value: string | undefined): string {
  if (!value) return "";
  const decoded = decode(value);
  return STORE_CODE_PATTERN.test(decoded) ? decoded : "";
}

function validTableScene(value: string | undefined): string {
  if (!value) return "";
  const decoded = decode(value).trim();
  return TABLE_SCENE_PATTERN.test(decoded) ? decoded : "";
}

function validFastFoodPublicId(value: string | undefined): string {
  if (!value) return "";
  const decoded = decode(value).trim();
  return /^[a-fA-F0-9]{28}$/.test(decoded) ? decoded.toLowerCase() : "";
}

function sceneFields(rawScene: string): Record<string, string> {
  const scene = decode(rawScene).replace(/^\?/, "");
  return scene.split("&").reduce<Record<string, string>>((result, field) => {
    const separator = field.indexOf("=");
    if (separator < 0) return result;
    const key = decode(field.slice(0, separator));
    result[key] = decode(field.slice(separator + 1));
    return result;
  }, {});
}

function storeCodeFromScene(rawScene: string | undefined): string {
  if (!rawScene) return "";
  const scene = decode(rawScene);
  if (!scene.includes("=")) return validStoreCode(scene);
  const fields = sceneFields(scene);
  return validStoreCode(fields.storeCode) || validStoreCode(fields.store) || validStoreCode(fields.s);
}

/** Returns an explicitly routed store, or an empty string when this route has none. */
export function routedStoreCode(query: StoreRouteQuery): string {
  return validStoreCode(query.storeCode)
    || validStoreCode(query.store)
    || validStoreCode(query.s)
    || storeCodeFromScene(query.scene);
}

function presentValue(query: StoreRouteQuery, keys: readonly string[]): string | undefined {
  for (const key of keys) {
    if (Object.prototype.hasOwnProperty.call(query, key)) return query[key];
  }
  return undefined;
}

function hasAnyKey(query: StoreRouteQuery, keys: readonly string[]): boolean {
  return keys.some((key) => Object.prototype.hasOwnProperty.call(query, key));
}

/**
 * Parses only routing parameters owned by Tanban. Custom QR payload lives in
 * `options.query.scene`; WeChat's numeric `options.scene` is deliberately ignored.
 */
export function parseOrderingEntry(options: OrderingEntryOptions): OrderingEntryRoute {
  const query = options.query || {};
  const storeKeys = ["storeCode", "store", "s"] as const;
  const hasExplicitStore = hasAnyKey(query, storeKeys);
  const rawStore = presentValue(query, storeKeys);
  const explicitStore = validStoreCode(rawStore);
  if (hasExplicitStore && !explicitStore) {
    return { kind: "INVALID", message: INVALID_QR_MESSAGE };
  }

  if (hasAnyKey(query, TABLE_KEYS)) {
    const publicScene = validTableScene(presentValue(query, TABLE_KEYS));
    if (!publicScene) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
    return { kind: "TABLE", publicScene, ...(explicitStore ? { expectedStoreCode: explicitStore } : {}) };
  }

  if (hasAnyKey(query, FAST_FOOD_KEYS)) {
    const publicId = validFastFoodPublicId(presentValue(query, FAST_FOOD_KEYS));
    if (!publicId) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
    return { kind: "FAST_FOOD", publicId, ...(explicitStore ? { expectedStoreCode: explicitStore } : {}) };
  }

  if (Object.prototype.hasOwnProperty.call(query, "scene")) {
    const rawScene = query.scene;
    if (!rawScene) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
    const scene = decode(rawScene).trim();
    if (!scene) return { kind: "INVALID", message: INVALID_QR_MESSAGE };

    if (scene.includes("=")) {
      const fields = sceneFields(scene);
      if (hasAnyKey(fields, TABLE_KEYS)) {
        const publicScene = validTableScene(presentValue(fields, TABLE_KEYS));
        if (!publicScene) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
        const sceneStore = validStoreCode(fields.storeCode) || validStoreCode(fields.store) || validStoreCode(fields.s);
        if (hasAnyKey(fields, storeKeys) && !sceneStore) {
          return { kind: "INVALID", message: INVALID_QR_MESSAGE };
        }
        if (explicitStore && sceneStore && explicitStore !== sceneStore) {
          return { kind: "INVALID", message: WRONG_STORE_QR_MESSAGE };
        }
        const expectedStoreCode = explicitStore || sceneStore;
        return { kind: "TABLE", publicScene, ...(expectedStoreCode ? { expectedStoreCode } : {}) };
      }
      if (hasAnyKey(fields, FAST_FOOD_KEYS)) {
        const publicId = validFastFoodPublicId(presentValue(fields, FAST_FOOD_KEYS));
        if (!publicId) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
        const sceneStore = validStoreCode(fields.storeCode) || validStoreCode(fields.store) || validStoreCode(fields.s);
        if (hasAnyKey(fields, storeKeys) && !sceneStore) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
        if (explicitStore && sceneStore && explicitStore !== sceneStore) return { kind: "INVALID", message: WRONG_STORE_QR_MESSAGE };
        const expectedStoreCode = explicitStore || sceneStore;
        return { kind: "FAST_FOOD", publicId, ...(expectedStoreCode ? { expectedStoreCode } : {}) };
      }
      const sceneStore = validStoreCode(fields.storeCode) || validStoreCode(fields.store) || validStoreCode(fields.s);
      if (!sceneStore) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
      if (explicitStore && explicitStore !== sceneStore) {
        return { kind: "INVALID", message: WRONG_STORE_QR_MESSAGE };
      }
      return { kind: "STORE", storeCode: explicitStore || sceneStore };
    }

    // Backward-compatible unlimited-code payloads may be the bare opaque token.
    // Limit this heuristic to 20-32 hex chars so ordinary human-readable stores
    // keep their legacy raw-scene behavior.
    const bareTableScene = /^[a-fA-F0-9]{20,32}$/.test(scene) ? validTableScene(scene) : "";
    if (bareTableScene) {
      return { kind: "TABLE", publicScene: bareTableScene, ...(explicitStore ? { expectedStoreCode: explicitStore } : {}) };
    }
    const sceneStore = validStoreCode(scene);
    if (!sceneStore) return { kind: "INVALID", message: INVALID_QR_MESSAGE };
    if (explicitStore && explicitStore !== sceneStore) {
      return { kind: "INVALID", message: WRONG_STORE_QR_MESSAGE };
    }
    return { kind: "STORE", storeCode: explicitStore || sceneStore };
  }

  if (explicitStore) return { kind: "STORE", storeCode: explicitStore };
  return { kind: "NONE" };
}

export function orderingEntryKey(route: OrderingEntryRoute): string {
  if (route.kind === "TABLE") return `TABLE:${route.expectedStoreCode || ""}:${route.publicScene}`;
  if (route.kind === "FAST_FOOD") return `FAST_FOOD:${route.expectedStoreCode || ""}:${route.publicId}`;
  if (route.kind === "STORE") return `STORE:${route.storeCode}`;
  return route.kind;
}
