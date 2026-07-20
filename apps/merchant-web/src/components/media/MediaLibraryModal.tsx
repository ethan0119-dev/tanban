import {
  CheckCircleFilled,
  CloudUploadOutlined,
  FolderAddOutlined,
  InboxOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Empty, Form, Image, Input, Modal, Spin, Tag, Typography, Upload, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../../api/client';
import { mediaApi } from '../../features/media/api';
import type { MediaAsset, MediaGroup } from '../../features/media/model';
import type { Id } from '../../types';
import './media-library.css';

interface MediaLibraryModalProps {
  open: boolean;
  title?: string;
  multiple?: boolean;
  maxSelection?: number;
  excludeUrls?: string[];
  onCancel: () => void;
  onConfirm: (assets: MediaAsset[]) => void;
}

export function MediaLibraryModal({
  open,
  title = '从图片库选择',
  multiple = false,
  maxSelection = 1,
  excludeUrls = [],
  onCancel,
  onConfirm,
}: MediaLibraryModalProps) {
  const [groups, setGroups] = useState<MediaGroup[]>([]);
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [allAssetCount, setAllAssetCount] = useState(0);
  const [groupId, setGroupId] = useState<Id | 'ALL'>('ALL');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [selected, setSelected] = useState<MediaAsset[]>([]);
  const [groupModalOpen, setGroupModalOpen] = useState(false);
  const [groupSaving, setGroupSaving] = useState(false);
  const [groupForm] = Form.useForm<{ name: string }>();
  const [messageApi, holder] = message.useMessage();

  const loadGroups = useCallback(async () => {
    try {
      setGroups(await mediaApi.listGroups());
    } catch (error) {
      messageApi.warning(`图片分组加载失败：${errorMessage(error)}`);
    }
  }, [messageApi]);

  const loadAssets = useCallback(async () => {
    if (!open) return;
    setLoading(true);
    try {
      const result = await mediaApi.listAssets({
        keyword: keyword.trim() || undefined,
        groupId: groupId === 'ALL' ? undefined : groupId,
        pageSize: 100,
      });
      setAssets(result.items.filter((item) => item.type === 'IMAGE' && item.url && !excludeUrls.includes(item.url)));
      if (groupId === 'ALL' && !keyword.trim()) setAllAssetCount(Number(result.meta.total ?? result.items.length));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [excludeUrls, groupId, keyword, messageApi, open]);

  useEffect(() => {
    if (!open) return;
    setSelected([]);
    setGroupId('ALL');
    setKeyword('');
    void loadGroups();
  }, [loadGroups, open]);

  useEffect(() => { void loadAssets(); }, [loadAssets]);

  const selectedIDs = useMemo(() => new Set(selected.map((item) => String(item.id))), [selected]);
  const toggle = (asset: MediaAsset) => {
    if (!multiple) {
      setSelected([asset]);
      return;
    }
    setSelected((current) => {
      if (current.some((item) => String(item.id) === String(asset.id))) return current.filter((item) => String(item.id) !== String(asset.id));
      if (current.length >= maxSelection) {
        messageApi.warning(`最多选择 ${maxSelection} 张图片`);
        return current;
      }
      return [...current, asset];
    });
  };

  const upload = async (file: File) => {
    if (file.size > 8 * 1024 * 1024) {
      messageApi.error('单张图片不能超过 8MB');
      throw new Error('图片过大');
    }
    setUploading(true);
    try {
      const created = await mediaApi.uploadAsset(file, { groupId: groupId === 'ALL' ? undefined : groupId });
      setAssets((items) => [created, ...items.filter((item) => String(item.id) !== String(created.id))]);
      toggle(created);
      messageApi.success('图片已上传到当前商户图片库');
      await loadGroups();
      return created;
    } catch (error) {
      messageApi.error(errorMessage(error));
      throw error;
    } finally {
      setUploading(false);
    }
  };

  const createGroup = async () => {
    const values = await groupForm.validateFields();
    setGroupSaving(true);
    try {
      const created = await mediaApi.createGroup({ name: values.name.trim() });
      setGroups((items) => [...items, created]);
      setGroupId(created.id);
      setGroupModalOpen(false);
      groupForm.resetFields();
      messageApi.success('图片分组已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setGroupSaving(false);
    }
  };

  return (
    <>
      {holder}
      <Modal
        className="media-library-modal"
        width={1080}
        title={<div><strong>{title}</strong><Typography.Text type="secondary"> 统一素材可在商品、装修和营销中重复使用</Typography.Text></div>}
        open={open}
        onCancel={onCancel}
        okText={multiple ? `使用所选图片（${selected.length}/${maxSelection}）` : '使用这张图片'}
        okButtonProps={{ disabled: selected.length === 0 }}
        onOk={() => onConfirm(selected)}
        destroyOnHidden
      >
        <div className="media-picker-layout">
          <aside className="media-group-sidebar">
            <button type="button" className={groupId === 'ALL' ? 'active' : ''} onClick={() => setGroupId('ALL')}>
              <span>全部图片</span><em>{allAssetCount || '—'}</em>
            </button>
            {groups.map((group) => (
              <button type="button" key={String(group.id)} className={String(groupId) === String(group.id) ? 'active' : ''} onClick={() => setGroupId(group.id)}>
                <span>{group.name}</span><em>{group.assetCount}</em>
              </button>
            ))}
            <Button block type="dashed" icon={<FolderAddOutlined />} onClick={() => setGroupModalOpen(true)}>新建分组</Button>
          </aside>
          <section className="media-picker-main">
            <div className="media-picker-toolbar">
              <Input.Search allowClear prefix={<SearchOutlined />} value={keyword} placeholder="搜索图片名称" onChange={(event) => setKeyword(event.target.value)} onSearch={() => void loadAssets()} />
              <Upload
                accept="image/jpeg,image/png,image/gif"
                multiple
                showUploadList={false}
                customRequest={({ file, onError, onSuccess }) => void upload(file as File).then((asset) => onSuccess?.(asset)).catch((error: Error) => onError?.(error))}
              >
                <Button type="primary" loading={uploading} icon={<CloudUploadOutlined />}>上传图片</Button>
              </Upload>
            </div>
            <Spin spinning={loading}>
              {assets.length ? (
                <div className="media-picker-grid">
                  {assets.map((asset) => {
                    const checked = selectedIDs.has(String(asset.id));
                    return (
                      <button type="button" key={String(asset.id)} className={`media-picker-card ${checked ? 'selected' : ''}`} onClick={() => toggle(asset)}>
                        <span className="media-picker-image"><Image preview={false} src={asset.url} alt={asset.name} fallback={transparentPixel} /></span>
                        <span className="media-picker-name" title={asset.name}>{asset.name}</span>
                        <span className="media-picker-meta">{asset.width && asset.height ? `${asset.width} × ${asset.height}` : asset.groupName || '未分组'}</span>
                        {checked && <CheckCircleFilled className="media-picker-check" />}
                      </button>
                    );
                  })}
                </div>
              ) : <Empty image={<InboxOutlined />} description={keyword ? '没有匹配的图片' : '当前分组还没有图片'} />}
            </Spin>
            {multiple && <div className="media-selection-strip"><Typography.Text type="secondary">已选</Typography.Text>{selected.map((asset) => <Tag key={String(asset.id)} closable onClose={() => toggle(asset)}>{asset.name}</Tag>)}</div>}
          </section>
        </div>
      </Modal>
      <Modal title="新建图片分组" open={groupModalOpen} okText="创建分组" confirmLoading={groupSaving} onCancel={() => setGroupModalOpen(false)} onOk={() => void createGroup()}>
        <Form form={groupForm} layout="vertical"><Form.Item name="name" label="分组名称" rules={[{ required: true, whitespace: true, message: '请输入分组名称' }, { max: 40 }]}><Input autoFocus placeholder="例如：商品主图、首页装修" /></Form.Item></Form>
      </Modal>
    </>
  );
}

const transparentPixel = 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=';
