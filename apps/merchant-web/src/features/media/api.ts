import { api } from '../../api/client';
import type { Id, ListResult } from '../../types';
import { normalizeMediaAsset, normalizeMediaGroup, type MediaAsset, type MediaAssetQuery, type MediaGroup } from './model';

function encode(id: Id) {
  return encodeURIComponent(String(id));
}

export const mediaApi = {
  async listGroups(): Promise<MediaGroup[]> {
    const result = await api.getList<unknown>('/merchant/media-groups');
    return result.items.map(normalizeMediaGroup);
  },

  async createGroup(input: { name: string; sortOrder?: number }): Promise<MediaGroup> {
    return normalizeMediaGroup(await api.post('/merchant/media-groups', {
      name: input.name,
      sort_order: input.sortOrder ?? 0,
    }));
  },

  async updateGroup(id: Id, input: { name: string; sortOrder?: number }): Promise<MediaGroup> {
    return normalizeMediaGroup(await api.put(`/merchant/media-groups/${encode(id)}`, {
      name: input.name,
      sort_order: input.sortOrder ?? 0,
    }));
  },

  deleteGroup(id: Id): Promise<unknown> {
    return api.delete(`/merchant/media-groups/${encode(id)}`);
  },

  async listAssets(query: MediaAssetQuery = {}): Promise<ListResult<MediaAsset>> {
    const result = await api.getList<unknown>('/merchant/media-assets', {
      q: query.keyword || undefined,
      group_id: query.groupId || undefined,
      page: query.page ?? 1,
      page_size: query.pageSize ?? 60,
    });
    return { items: result.items.map(normalizeMediaAsset), meta: result.meta };
  },

  async uploadAsset(file: File, input: { name?: string; groupId?: Id } = {}): Promise<MediaAsset> {
    const form = new FormData();
    form.append('file', file);
    form.append('name', input.name || file.name);
    if (input.groupId !== undefined && input.groupId !== '') form.append('group_id', String(input.groupId));
    return normalizeMediaAsset(await api.postForm('/merchant/media-assets/upload', form));
  },

  async updateAsset(asset: MediaAsset, input: { name: string; groupId?: Id }): Promise<MediaAsset> {
    return normalizeMediaAsset(await api.put(`/merchant/media-assets/${encode(asset.id)}`, {
      name: input.name,
      group_id: input.groupId ? Number(input.groupId) : 0,
      url: asset.url,
      storageKey: asset.storageKey || '',
      mimeType: asset.mimeType || '',
      width: asset.width || 0,
      height: asset.height || 0,
      sizeBytes: asset.sizeBytes || 0,
    }));
  },

  deleteAsset(id: Id): Promise<unknown> {
    return api.delete(`/merchant/media-assets/${encode(id)}`);
  },
};
