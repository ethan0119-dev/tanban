import type { Id } from '../../types';

export interface MediaGroup {
  id: Id;
  name: string;
  sortOrder: number;
  assetCount: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface MediaAsset {
  id: Id;
  name: string;
  url: string;
  type: 'IMAGE' | 'VIDEO';
  groupId?: Id;
  groupName?: string;
  mimeType?: string;
  storageKey?: string;
  width?: number;
  height?: number;
  sizeBytes?: number;
  createdAt?: string;
  updatedAt?: string;
}

export interface MediaAssetQuery {
  keyword?: string;
  groupId?: Id;
  page?: number;
  pageSize?: number;
}

export function normalizeMediaGroup(payload: unknown): MediaGroup {
  const value = record(payload);
  return {
    id: (value.id as Id | undefined) ?? '',
    name: text(value.name) || '未命名分组',
    sortOrder: integer(value.sortOrder, value.sort_order),
    assetCount: integer(value.assetCount, value.asset_count),
    createdAt: text(value.createdAt, value.created_at),
    updatedAt: text(value.updatedAt, value.updated_at),
  };
}

export function normalizeMediaAsset(payload: unknown): MediaAsset {
  const value = record(payload);
  const rawType = text(value.type, value.kind).toUpperCase();
  return {
    id: (value.id as Id | undefined) ?? '',
    name: text(value.name) || '未命名图片',
    url: text(value.url),
    type: rawType === 'VIDEO' ? 'VIDEO' : 'IMAGE',
    groupId: (value.groupId ?? value.group_id) as Id | undefined,
    groupName: text(value.groupName, value.group_name),
    mimeType: text(value.mimeType, value.mime_type),
    storageKey: text(value.storageKey, value.storage_key),
    width: integer(value.width) || undefined,
    height: integer(value.height) || undefined,
    sizeBytes: integer(value.sizeBytes, value.size_bytes) || undefined,
    createdAt: text(value.createdAt, value.created_at),
    updatedAt: text(value.updatedAt, value.updated_at),
  };
}

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function text(...values: unknown[]): string {
  return String(values.find((value) => typeof value === 'string') ?? '');
}

function integer(...values: unknown[]): number {
  const value = values.find((item) => item !== undefined && item !== null && Number.isFinite(Number(item)));
  return value === undefined ? 0 : Number(value);
}
