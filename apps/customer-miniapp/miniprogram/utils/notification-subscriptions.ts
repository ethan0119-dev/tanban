import { env } from "../config/env";
import { request } from "./request";

export type NotificationScene = "PICKUP_READY" | "RECHARGE_SUCCESS" | "BALANCE_CONSUMED";

interface NotificationTemplate {
  scene: NotificationScene;
  templateId: string;
  title: string;
}

interface NotificationTemplateView {
  available: boolean;
  onboardingRequested: boolean;
  templates: NotificationTemplate[];
}

interface SubscriptionResult {
  scene: NotificationScene;
  templateId: string;
  result: "accept" | "reject" | "ban" | "filter";
}

interface SubscriptionRecord {
  storeCode: string;
  requestId: string;
  requestContext: string;
  businessNo: string;
  results: SubscriptionResult[];
}

interface SubscriptionOptions {
  storeCode: string;
  scenes: NotificationScene[];
  requestContext: "ORDER" | "RECHARGE";
  businessNo?: string;
  includeOnboarding?: boolean;
}

const requestMarkersKey = "tanban_notification_subscription_requests_v1";
const pendingRecordsKey = "tanban_notification_subscription_pending_v1";
const markerTTL = 30 * 24 * 60 * 60 * 1000;

function requestSubscribeMessage(templateIds: string[]): Promise<Record<string, string>> {
  return new Promise((resolve, reject) => {
    wx.requestSubscribeMessage({
      tmplIds: templateIds,
      success(result) {
        resolve(result as unknown as Record<string, string>);
      },
      fail(error) {
        reject(new Error(error.errMsg || "订阅消息授权未完成"));
      },
    });
  });
}

function readMarkers(): Record<string, number> {
  const stored = wx.getStorageSync<Record<string, number>>(requestMarkersKey);
  const now = Date.now();
  return Object.fromEntries(Object.entries(stored || {}).filter(([, requestedAt]) => now - Number(requestedAt) < markerTTL));
}

function markerKey(scene: NotificationScene, businessNo: string): string {
  return `${env.channelKey}:${scene}:${businessNo}`;
}

function rememberRequested(scenes: NotificationScene[], businessNo: string) {
  if (!businessNo) return;
  const markers = readMarkers();
  const now = Date.now();
  for (const scene of scenes) markers[markerKey(scene, businessNo)] = now;
  wx.setStorageSync(requestMarkersKey, markers);
}

function pendingRecords(): SubscriptionRecord[] {
  return wx.getStorageSync<SubscriptionRecord[]>(pendingRecordsKey) || [];
}

async function postRecord(record: SubscriptionRecord): Promise<void> {
  await request({
    url: `/public/stores/${encodeURIComponent(record.storeCode)}/notification-subscriptions/results`,
    method: "POST",
    data: {
      requestId: record.requestId,
      requestContext: record.requestContext,
      businessNo: record.businessNo,
      results: record.results,
    },
  });
}

async function flushPendingRecords(storeCode: string): Promise<void> {
  const pending = pendingRecords();
  if (!pending.length) return;
  const delivered = new Set<string>();
  for (const record of pending) {
    if (record.storeCode !== storeCode) continue;
    try {
      await postRecord(record);
      delivered.add(record.requestId);
    } catch {
      // Keep the record for a later visit to the same store.
    }
  }
  if (delivered.size) {
    wx.setStorageSync(pendingRecordsKey, pendingRecords().filter((record) => !delivered.has(record.requestId)).slice(-20));
  }
}

function queueRecord(record: SubscriptionRecord) {
  const pending = pendingRecords().filter((item) => item.requestId !== record.requestId);
  pending.push(record);
  wx.setStorageSync(pendingRecordsKey, pending.slice(-20));
}

function newRequestId(): string {
  return `sub_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export async function requestNotificationSubscriptions(options: SubscriptionOptions): Promise<SubscriptionResult[]> {
  void flushPendingRecords(options.storeCode);
  let view: NotificationTemplateView;
  try {
    view = await request<NotificationTemplateView>({
      url: `/public/stores/${encodeURIComponent(options.storeCode)}/notification-subscriptions/templates`,
      method: "GET",
    });
  } catch (error) {
    console.warn("notification templates unavailable", error);
    return [];
  }
  if (!view.available || !view.templates?.length) return [];

  const requestedScenes = options.includeOnboarding && !view.onboardingRequested
    ? new Set<NotificationScene>(["PICKUP_READY", "RECHARGE_SUCCESS", "BALANCE_CONSUMED"])
    : new Set(options.scenes);
  const markers = readMarkers();
  const templates = view.templates.filter((template) => requestedScenes.has(template.scene))
    .filter((template) => !options.businessNo || !markers[markerKey(template.scene, options.businessNo)]);
  if (!templates.length) return [];

  let rawResults: Record<string, string>;
  try {
    rawResults = await requestSubscribeMessage(templates.map((template) => template.templateId));
  } catch (error) {
    console.warn("notification subscription request was not completed", error);
    return [];
  }
  const results = templates.map<SubscriptionResult>((template) => {
    const raw = String(rawResults[template.templateId] || "reject").toLowerCase();
    const result = (["accept", "reject", "ban", "filter"].includes(raw) ? raw : "reject") as SubscriptionResult["result"];
    return { scene: template.scene, templateId: template.templateId, result };
  });
  const record: SubscriptionRecord = {
    storeCode: options.storeCode,
    requestId: newRequestId(),
    requestContext: options.requestContext,
    businessNo: options.businessNo || "",
    results,
  };
  rememberRequested(templates.map((template) => template.scene), options.businessNo || "");
  queueRecord(record);
  try {
    await postRecord(record);
    wx.setStorageSync(pendingRecordsKey, pendingRecords().filter((item) => item.requestId !== record.requestId));
  } catch (error) {
    console.warn("notification subscription result will retry later", error);
  }
  return results;
}
