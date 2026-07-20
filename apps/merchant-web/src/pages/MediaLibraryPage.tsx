import {
  CloudUploadOutlined,
  DeleteOutlined,
  EditOutlined,
  FolderAddOutlined,
  InboxOutlined,
  PictureOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Empty, Form, Image, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Spin, Statistic, Typography, Upload, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { mediaApi } from '../features/media/api';
import type { MediaAsset, MediaGroup } from '../features/media/model';
import type { Id } from '../types';
import '../components/media/media-library.css';

interface GroupFormValues { name: string; sortOrder: number }
interface AssetFormValues { name: string; groupId?: Id }

export function MediaLibraryPage() {
  const [groups, setGroups] = useState<MediaGroup[]>([]);
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [allAssetCount, setAllAssetCount] = useState(0);
  const [groupId, setGroupId] = useState<Id | 'ALL'>('ALL');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [groupEditorOpen, setGroupEditorOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<MediaGroup>();
  const [assetEditorOpen, setAssetEditorOpen] = useState(false);
  const [editingAsset, setEditingAsset] = useState<MediaAsset>();
  const [groupForm] = Form.useForm<GroupFormValues>();
  const [assetForm] = Form.useForm<AssetFormValues>();
  const [messageApi, holder] = message.useMessage();

  const loadGroups = useCallback(async () => {
    try {
      setGroups(await mediaApi.listGroups());
    } catch (error) {
      messageApi.warning(`图片分组加载失败：${errorMessage(error)}`);
    }
  }, [messageApi]);

  const loadAssets = useCallback(async () => {
    setLoading(true);
    try {
      const result = await mediaApi.listAssets({ keyword: keyword.trim() || undefined, groupId: groupId === 'ALL' ? undefined : groupId, pageSize: 100 });
      setAssets(result.items.filter((item) => item.type === 'IMAGE'));
      if (groupId === 'ALL' && !keyword.trim()) setAllAssetCount(Number(result.meta.total ?? result.items.length));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [groupId, keyword, messageApi]);

  const load = useCallback(async () => { await Promise.all([loadGroups(), loadAssets()]); }, [loadAssets, loadGroups]);
  useEffect(() => { void load(); }, [load]);

  const openGroup = (group?: MediaGroup) => {
    setEditingGroup(group);
    groupForm.setFieldsValue({ name: group?.name || '', sortOrder: group?.sortOrder ?? groups.length + 1 });
    setGroupEditorOpen(true);
  };

  const saveGroup = async () => {
    const values = await groupForm.validateFields();
    setSaving(true);
    try {
      const saved = editingGroup
        ? await mediaApi.updateGroup(editingGroup.id, { name: values.name.trim(), sortOrder: values.sortOrder })
        : await mediaApi.createGroup({ name: values.name.trim(), sortOrder: values.sortOrder });
      setGroups((items) => editingGroup ? items.map((item) => String(item.id) === String(saved.id) ? saved : item) : [...items, saved]);
      setGroupEditorOpen(false);
      messageApi.success(editingGroup ? '分组已更新' : '分组已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const deleteGroup = async (group: MediaGroup) => {
    try {
      await mediaApi.deleteGroup(group.id);
      if (String(groupId) === String(group.id)) setGroupId('ALL');
      messageApi.success('分组已删除，原图片会保留在未分组中');
      await loadGroups();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const upload = async (file: File) => {
    if (file.size > 8 * 1024 * 1024) {
      messageApi.error('单张图片不能超过 8MB');
      throw new Error('图片过大');
    }
    setUploading(true);
    try {
      const asset = await mediaApi.uploadAsset(file, { groupId: groupId === 'ALL' ? undefined : groupId });
      setAssets((items) => [{ ...asset, groupName: groupId === 'ALL' ? asset.groupName : groups.find((group) => String(group.id) === String(groupId))?.name }, ...items]);
      messageApi.success(`${file.name} 已上传`);
      await Promise.all([loadGroups(), loadAssets()]);
      return asset;
    } catch (error) {
      messageApi.error(errorMessage(error));
      throw error;
    } finally {
      setUploading(false);
    }
  };

  const openAsset = (asset: MediaAsset) => {
    setEditingAsset(asset);
    assetForm.setFieldsValue({ name: asset.name, groupId: asset.groupId });
    setAssetEditorOpen(true);
  };

  const saveAsset = async () => {
    if (!editingAsset) return;
    const values = await assetForm.validateFields();
    setSaving(true);
    try {
      const updated = await mediaApi.updateAsset(editingAsset, { name: values.name.trim(), groupId: values.groupId });
      setAssets((items) => items.map((item) => String(item.id) === String(updated.id) ? { ...updated, groupName: groups.find((group) => String(group.id) === String(values.groupId))?.name } : item));
      setAssetEditorOpen(false);
      messageApi.success('图片信息已更新');
      await Promise.all([loadGroups(), loadAssets()]);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const deleteAsset = async (asset: MediaAsset) => {
    try {
      await mediaApi.deleteAsset(asset.id);
      setAssets((items) => items.filter((item) => String(item.id) !== String(asset.id)));
      messageApi.success('图片已从素材库删除');
      await loadGroups();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const totalAssets = allAssetCount;
  const selectedGroupName = groupId === 'ALL' ? '全部图片' : groups.find((item) => String(item.id) === String(groupId))?.name || '图片分组';

  return (
    <div className="page-shell media-library-page">
      {holder}
      <PageHeading
        title="图片库"
        description="统一管理商户图片素材；商品、店铺装修、热区和营销广告可直接复用"
        extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Upload accept="image/jpeg,image/png,image/gif" multiple showUploadList={false} customRequest={({ file, onSuccess, onError }) => void upload(file as File).then((asset) => onSuccess?.(asset)).catch((error: Error) => onError?.(error))}><Button type="primary" loading={uploading} icon={<CloudUploadOutlined />}>上传图片</Button></Upload></Space>}
      />
      <Row gutter={[16, 16]} className="media-library-summary">
        <Col xs={12} md={6}><Card bordered={false}><Statistic title="图片总数" value={totalAssets || assets.length} prefix={<PictureOutlined />} /></Card></Col>
        <Col xs={12} md={6}><Card bordered={false}><Statistic title="图片分组" value={groups.length} /></Card></Col>
      </Row>
      <div className="media-picker-layout">
        <aside className="media-group-sidebar">
          <button type="button" className={groupId === 'ALL' ? 'active' : ''} onClick={() => setGroupId('ALL')}><span>全部图片</span><em>{totalAssets || '—'}</em></button>
          {groups.map((group) => (
            <div key={String(group.id)} className="media-group-admin-row">
              <button type="button" className={String(groupId) === String(group.id) ? 'active' : ''} onClick={() => setGroupId(group.id)}><span>{group.name}</span><em>{group.assetCount}</em></button>
              <Space size={0}>
                <Button size="small" type="text" icon={<EditOutlined />} aria-label={`编辑${group.name}`} onClick={() => openGroup(group)} />
                <Popconfirm title={`删除“${group.name}”？`} description="图片不会被删除，只会移动到未分组。" onConfirm={() => void deleteGroup(group)}><Button size="small" type="text" danger icon={<DeleteOutlined />} aria-label={`删除${group.name}`} /></Popconfirm>
              </Space>
            </div>
          ))}
          <Button block type="dashed" icon={<FolderAddOutlined />} onClick={() => openGroup()}>新建分组</Button>
        </aside>
        <section className="media-picker-main">
          <div className="media-library-toolbar">
            <Input.Search allowClear prefix={<SearchOutlined />} value={keyword} placeholder="按图片名称搜索" onChange={(event) => setKeyword(event.target.value)} onSearch={() => void loadAssets()} />
            <Typography.Text type="secondary">{selectedGroupName} · 当前 {assets.length} 张</Typography.Text>
          </div>
          <Spin spinning={loading}>
            {assets.length ? <div className="media-picker-grid media-library-grid">
              {assets.map((asset) => <div key={String(asset.id)} className="media-picker-card">
                <span className="media-picker-image"><Image src={asset.url} alt={asset.name} fallback={transparentPixel} /></span>
                <span className="media-picker-name" title={asset.name}>{asset.name}</span>
                <span className="media-picker-meta">{asset.groupName || '未分组'}{asset.width && asset.height ? ` · ${asset.width}×${asset.height}` : ''}</span>
                <div className="asset-management-actions"><Button size="small" icon={<EditOutlined />} onClick={() => openAsset(asset)}>编辑</Button><Popconfirm title="删除这张图片？" description="正在被商品、装修、营销或门店使用的图片不可删除，请先解除引用。" onConfirm={() => void deleteAsset(asset)}><Button size="small" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm></div>
              </div>)}
            </div> : <Empty image={<InboxOutlined />} description={keyword ? '没有匹配的图片' : '当前分组还没有图片'} />}
          </Spin>
        </section>
      </div>

      <Modal title={editingGroup ? '编辑图片分组' : '新建图片分组'} open={groupEditorOpen} confirmLoading={saving} okText="保存分组" onCancel={() => setGroupEditorOpen(false)} onOk={() => void saveGroup()} destroyOnHidden>
        <Form form={groupForm} layout="vertical"><Form.Item name="name" label="分组名称" rules={[{ required: true, whitespace: true }, { max: 40 }]}><Input placeholder="例如：商品主图" /></Form.Item><Form.Item name="sortOrder" label="排序"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Form>
      </Modal>
      <Modal title="编辑图片" open={assetEditorOpen} confirmLoading={saving} okText="保存修改" onCancel={() => setAssetEditorOpen(false)} onOk={() => void saveAsset()} destroyOnHidden>
        <Form form={assetForm} layout="vertical"><Form.Item name="name" label="图片名称" rules={[{ required: true, whitespace: true }, { max: 100 }]}><Input /></Form.Item><Form.Item name="groupId" label="所属分组"><Select allowClear placeholder="未分组" options={groups.map((group) => ({ value: group.id, label: group.name }))} /></Form.Item></Form>
      </Modal>
    </div>
  );
}

const transparentPixel = 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=';
