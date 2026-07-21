import {
  AppstoreOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  Form,
  Image,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { ImagePickerField } from '../components/media/ImagePickerField';
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import { merchantFeatureCopy } from '../features/availability/copy';
import './catalog.css';

type ResourceType = 'PACKAGE' | 'TEMP_PRODUCT' | 'UNIT' | 'PRODUCT_TAG' | 'PRINT_LABEL' | 'NOTE' | 'SPEC_TEMPLATE' | 'ATTRIBUTE_TEMPLATE' | 'MODIFIER_TEMPLATE';

interface CategoryRow { id: number; name: string; sort_order: number; status: string }
interface ResourceRow {
  id: number;
  resource_type: ResourceType;
  code: string;
  name: string;
  description: string;
  price_cents: number;
  config: Record<string, unknown>;
  sort_order: number;
  status: string;
}
interface ModifierItem { id: number; name: string; price_cents: number; image_url: string; sort_order: number; status: string }
interface ModifierGroupItem { modifier_item_id: number; name: string; default_price_cents: number; price_cents: number; is_default: boolean; sort_order: number }
interface ModifierGroup { id: number; name: string; min_select: number; max_select: number; sort_order: number; status: string; items: ModifierGroupItem[] }

const resourceLabels: Record<ResourceType, string> = {
  PACKAGE: '套餐商品',
  TEMP_PRODUCT: '临时商品库',
  UNIT: '单位库',
  PRODUCT_TAG: '商品标签',
  PRINT_LABEL: '打印标签',
  NOTE: '商品备注库',
  SPEC_TEMPLATE: '规格模板库',
  ATTRIBUTE_TEMPLATE: '属性模板库',
  MODIFIER_TEMPLATE: '加料模板库',
};

const libraryTypes: ResourceType[] = ['UNIT', 'PRODUCT_TAG', 'PRINT_LABEL', 'NOTE'];
const templateTypes: ResourceType[] = ['SPEC_TEMPLATE', 'ATTRIBUTE_TEMPLATE', 'MODIFIER_TEMPLATE'];

function ResourcePanel({ type, resources, reload }: { type: ResourceType; resources: ResourceRow[]; reload: () => Promise<void> }) {
  const [form] = Form.useForm();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ResourceRow | null>(null);
  const [saving, setSaving] = useState(false);
  const [messageApi, holder] = message.useMessage();
  const rows = resources.filter((item) => item.resource_type === type);
  const openEditor = (item?: ResourceRow) => {
    form.resetFields();
    setEditing(item ?? null);
    form.setFieldsValue(item ? {
      ...item,
      price: item.price_cents / 100,
      enabled: item.status === 'ACTIVE',
      configText: JSON.stringify(item.config || {}, null, 2),
    } : { enabled: true, price: 0, sort_order: rows.length, configText: '{}' });
    setOpen(true);
  };
  const save = async () => {
    const values = await form.validateFields();
    let config: Record<string, unknown> = {};
    try { config = JSON.parse(values.configText || '{}') as Record<string, unknown>; } catch { messageApi.error('扩展配置必须是合法 JSON'); return; }
    setSaving(true);
    const payload = {
      resource_type: type,
      code: values.code || '',
      name: values.name,
      description: values.description || '',
      price_cents: Math.round(Number(values.price || 0) * 100),
      config,
      sort_order: Number(values.sort_order || 0),
      status: values.enabled ? 'ACTIVE' : 'DISABLED',
    };
    try {
      if (editing) await api.put(`/merchant/catalog-resources/${editing.id}`, payload);
      else await api.post('/merchant/catalog-resources', payload);
      messageApi.success(editing ? '已更新' : '已创建');
      setOpen(false);
      await reload();
    } catch (error) { messageApi.error(errorMessage(error)); } finally { setSaving(false); }
  };
  const remove = async (id: number) => {
    try { await api.delete(`/merchant/catalog-resources/${id}`); messageApi.success('已删除'); await reload(); } catch (error) { messageApi.error(errorMessage(error)); }
  };
  return <>
    {holder}
    <div className="catalog-type-tip">
      {type === 'PACKAGE' ? merchantFeatureCopy.CATALOG_PACKAGE_SALE.description :
        type === 'PRINT_LABEL' ? '定义商品打印标签名称与版式信息；绑定正式打印设备后即可按商品生成标签。' :
          `${resourceLabels[type]}可作为商品资料维护并绑定；不会自动改变商品选项或点单价格。`}
    </div>
    <div className="catalog-toolbar"><Typography.Text type="secondary">共 {rows.length} 条配置</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新增{resourceLabels[type]}</Button></div>
    <Table<ResourceRow> rowKey="id" dataSource={rows} pagination={{ pageSize: 10 }} columns={[
      { title: '名称', dataIndex: 'name', render: (value, row) => <Space direction="vertical" size={0}><Typography.Text strong>{value}</Typography.Text><Typography.Text type="secondary">{row.code || '未设置编码'}</Typography.Text></Space> },
      { title: '说明', dataIndex: 'description', ellipsis: true },
      { title: '价格', dataIndex: 'price_cents', width: 120, render: (value) => value ? `¥${(value / 100).toFixed(2)}` : '—' },
      { title: '状态', dataIndex: 'status', width: 90, render: (value) => <Tag color={value === 'ACTIVE' ? 'success' : 'default'}>{value === 'ACTIVE' ? '启用' : '停用'}</Tag> },
      { title: '操作', width: 150, render: (_, row) => <Space><Button type="link" icon={<EditOutlined />} onClick={() => openEditor(row)}>编辑</Button><Popconfirm title="确认删除？" onConfirm={() => void remove(row.id)}><Button type="link" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm></Space> },
    ]} />
    <Modal title={`${editing ? '编辑' : '新增'}${resourceLabels[type]}`} open={open} onCancel={() => setOpen(false)} onOk={() => void save()} confirmLoading={saving} width={620}>
      <Form form={form} layout="vertical">
        <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input /></Form.Item>
        <Space align="start" style={{ width: '100%' }}>
          <Form.Item label="业务编码" name="code"><Input placeholder="可选" /></Form.Item>
          <Form.Item label="参考价格" name="price"><InputNumber min={0} precision={2} prefix="¥" /></Form.Item>
          <Form.Item label="排序" name="sort_order"><InputNumber min={0} precision={0} /></Form.Item>
          <Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item>
        </Space>
        <Form.Item label="说明" name="description"><Input.TextArea rows={3} /></Form.Item>
        <Form.Item label="扩展配置（JSON）" name="configText" extra="用于套餐组成、模板字段等高级信息；留空时使用默认值。"><Input.TextArea className="catalog-config-json" rows={5} /></Form.Item>
      </Form>
    </Modal>
  </>;
}

export function CatalogConfigPage() {
  const [categories, setCategories] = useState<CategoryRow[]>([]);
  const [resources, setResources] = useState<ResourceRow[]>([]);
  const [modifierItems, setModifierItems] = useState<ModifierItem[]>([]);
  const [modifierGroups, setModifierGroups] = useState<ModifierGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [categoryForm] = Form.useForm();
  const [itemForm] = Form.useForm();
  const [groupForm] = Form.useForm();
  const [categoryOpen, setCategoryOpen] = useState(false);
  const [itemOpen, setItemOpen] = useState(false);
  const [itemLibraryOpen, setItemLibraryOpen] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [editingCategory, setEditingCategory] = useState<CategoryRow | null>(null);
  const [editingItem, setEditingItem] = useState<ModifierItem | null>(null);
  const [editingGroup, setEditingGroup] = useState<ModifierGroup | null>(null);
  const [libraryType, setLibraryType] = useState<ResourceType>('UNIT');
  const [templateType, setTemplateType] = useState<ResourceType>('SPEC_TEMPLATE');
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [categoryResult, resourceResult, itemResult, groupResult] = await Promise.all([
        api.getList<CategoryRow>('/merchant/categories'),
        api.getList<ResourceRow>('/merchant/catalog-resources'),
        api.getList<ModifierItem>('/merchant/modifier-items'),
        api.getList<ModifierGroup>('/merchant/modifier-groups'),
      ]);
      setCategories(categoryResult.items);
      setResources(resourceResult.items);
      setModifierItems(itemResult.items);
      setModifierGroups(groupResult.items);
    } catch (error) { messageApi.error(errorMessage(error)); } finally { setLoading(false); }
  }, [messageApi]);
  useEffect(() => { void load(); }, [load]);

  const openCategory = (row?: CategoryRow) => {
    categoryForm.resetFields();
    setEditingCategory(row ?? null);
    categoryForm.setFieldsValue(row ? { name: row.name, sort_order: row.sort_order, enabled: row.status === 'ACTIVE' } : { sort_order: categories.length, enabled: true });
    setCategoryOpen(true);
  };
  const saveCategory = async () => {
    const values = await categoryForm.validateFields();
    const payload = { name: values.name, sort_order: Number(values.sort_order || 0), status: values.enabled ? 'ACTIVE' : 'DISABLED' };
    try {
      if (editingCategory) await api.put(`/merchant/categories/${editingCategory.id}`, payload); else await api.post('/merchant/categories', payload);
      setCategoryOpen(false); messageApi.success('分类已保存'); await load();
    } catch (error) { messageApi.error(errorMessage(error)); }
  };
  const removeCategory = async (row: CategoryRow) => {
    try { await api.delete(`/merchant/categories/${row.id}`); messageApi.success('分类已删除'); await load(); } catch (error) { messageApi.error(errorMessage(error)); }
  };

  const openItem = (row?: ModifierItem) => {
    itemForm.resetFields();
    setEditingItem(row ?? null);
    itemForm.setFieldsValue(row ? { ...row, price: row.price_cents / 100, enabled: row.status === 'ACTIVE' } : { price: 0, sort_order: modifierItems.length, enabled: true });
    setItemOpen(true);
  };
  const saveItem = async () => {
    const values = await itemForm.validateFields();
    const payload = { name: values.name, price_cents: Math.round(Number(values.price) * 100), image_url: values.image_url || '', sort_order: Number(values.sort_order || 0), status: values.enabled ? 'ACTIVE' : 'DISABLED' };
    try {
      if (editingItem) await api.put(`/merchant/modifier-items/${editingItem.id}`, payload); else await api.post('/merchant/modifier-items', payload);
      setItemOpen(false); messageApi.success('加料已保存'); await load();
    } catch (error) { messageApi.error(errorMessage(error)); }
  };
  const removeItem = async (row: ModifierItem) => {
    try { await api.delete(`/merchant/modifier-items/${row.id}`); messageApi.success('加料已删除'); await load(); } catch (error) { messageApi.error(errorMessage(error)); }
  };

  const openGroup = (row?: ModifierGroup) => {
    groupForm.resetFields();
    setEditingGroup(row ?? null);
    groupForm.setFieldsValue(row ? {
      name: row.name, min_select: row.min_select, max_select: row.max_select, sort_order: row.sort_order,
      enabled: row.status === 'ACTIVE', item_ids: row.items.map((item) => item.modifier_item_id),
    } : { min_select: 0, max_select: 1, sort_order: modifierGroups.length, enabled: true, item_ids: [] });
    setGroupOpen(true);
  };
  const saveGroup = async () => {
    const values = await groupForm.validateFields();
    const selected: number[] = values.item_ids || [];
    const payload = {
      name: values.name, min_select: Number(values.min_select), max_select: Number(values.max_select), sort_order: Number(values.sort_order || 0), status: values.enabled ? 'ACTIVE' : 'DISABLED',
      items: selected.map((id, index) => {
        const prior = editingGroup?.items.find((item) => item.modifier_item_id === id);
        return {
          modifier_item_id: id,
          price_override_cents: prior && prior.price_cents !== prior.default_price_cents ? prior.price_cents : undefined,
          is_default: prior?.is_default ?? false,
          sort_order: index,
        };
      }),
    };
    try {
      if (editingGroup) await api.put(`/merchant/modifier-groups/${editingGroup.id}`, payload); else await api.post('/merchant/modifier-groups', payload);
      setGroupOpen(false); messageApi.success('加料组已保存'); await load();
    } catch (error) { messageApi.error(errorMessage(error)); }
  };
  const removeGroup = async (row: ModifierGroup) => {
    try { await api.delete(`/merchant/modifier-groups/${row.id}`); messageApi.success('加料组已删除'); await load(); } catch (error) { messageApi.error(errorMessage(error)); }
  };

  const categoryContent = <>
    <div className="catalog-toolbar"><Typography.Text type="secondary">分类决定小程序左侧菜单与商品归属</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openCategory()}>新增分类</Button></div>
    <Table<CategoryRow> rowKey="id" loading={loading} dataSource={categories} columns={[
      { title: '排序', dataIndex: 'sort_order', width: 90 }, { title: '分类名称', dataIndex: 'name' },
      { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={value === 'ACTIVE' ? 'success' : 'default'}>{value === 'ACTIVE' ? '启用' : '停用'}</Tag> },
      { title: '操作', width: 170, render: (_, row) => <Space><Button type="link" icon={<EditOutlined />} onClick={() => openCategory(row)}>编辑</Button><Popconfirm title="删除后分类不再公开，确认继续？" onConfirm={() => void removeCategory(row)}><Button type="link" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm></Space> },
    ]} />
  </>;

  const modifierItemContent = <>
    <div className="catalog-type-tip">{merchantFeatureCopy.MODIFIER_INVENTORY.description}</div>
    <div className="catalog-toolbar"><Typography.Text type="secondary">共 {modifierItems.length} 个加料</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openItem()}>新增加料</Button></div>
    <Table<ModifierItem> rowKey="id" dataSource={modifierItems} columns={[
      { title: '图片', dataIndex: 'image_url', width: 76, render: (value) => value ? <Image src={value} alt="加料图片" width={44} height={44} style={{ objectFit: 'cover', borderRadius: 8 }} /> : '—' },
      { title: '名称', dataIndex: 'name' }, { title: '加价', dataIndex: 'price_cents', render: (value) => `¥${(value / 100).toFixed(2)}` },
      { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === 'ACTIVE' ? 'success' : 'default'}>{value === 'ACTIVE' ? '启用' : '停用'}</Tag> },
      { title: '操作', render: (_, row) => <Space><Button type="link" onClick={() => openItem(row)}>编辑</Button><Popconfirm title="确认删除？" onConfirm={() => void removeItem(row)}><Button type="link" danger>删除</Button></Popconfirm></Space> },
    ]} />
  </>;

  const modifierGroupContent = <>
    <div className="catalog-type-tip">加料组定义“至少选几个、最多选几个”，绑定商品后小程序会按这里的规则展示，保存订单时会重新核对金额。</div>
    <div className="catalog-toolbar"><Typography.Text type="secondary">共 {modifierGroups.length} 个加料组</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openGroup()}>新增加料组</Button></div>
    <Table<ModifierGroup> rowKey="id" dataSource={modifierGroups} columns={[
      { title: '组名', dataIndex: 'name' }, { title: '选择规则', render: (_, row) => `${row.min_select}–${row.max_select} 项` },
      { title: '包含加料', render: (_, row) => <div className="modifier-items">{row.items.map((item) => <span className="modifier-item-chip" key={item.modifier_item_id}>{item.name} +¥{(item.price_cents / 100).toFixed(2)}</span>)}</div> },
      { title: '状态', dataIndex: 'status', render: (value) => <Tag color={value === 'ACTIVE' ? 'success' : 'default'}>{value === 'ACTIVE' ? '启用' : '停用'}</Tag> },
      { title: '操作', render: (_, row) => <Space><Button type="link" onClick={() => openGroup(row)}>编辑</Button><Popconfirm title="确认删除？" onConfirm={() => void removeGroup(row)}><Button type="link" danger>删除</Button></Popconfirm></Space> },
    ]} />
  </>;

  const tabs = useMemo(() => [
    { key: 'categories', label: '分类管理', children: categoryContent },
    { key: 'packages', label: '套餐商品', children: <ResourcePanel type="PACKAGE" resources={resources} reload={load} /> },
    { key: 'modifier-items', label: '加料商品', children: modifierItemContent },
    { key: 'modifier-groups', label: '加料组', children: modifierGroupContent },
    { key: 'libraries', label: '商品扩展库', children: <><Select value={libraryType} onChange={setLibraryType} options={libraryTypes.map((value) => ({ value, label: resourceLabels[value] }))} style={{ width: 220, marginBottom: 18 }} /><ResourcePanel type={libraryType} resources={resources} reload={load} /></> },
    { key: 'templates', label: '配置模板库', children: <><Select value={templateType} onChange={setTemplateType} options={templateTypes.map((value) => ({ value, label: resourceLabels[value] }))} style={{ width: 220, marginBottom: 18 }} /><ResourcePanel type={templateType} resources={resources} reload={load} /></> },
    { key: 'temporary', label: '临时商品库', children: <ResourcePanel type="TEMP_PRODUCT" resources={resources} reload={load} /> },
  ], [categories, libraryType, loading, modifierGroups, modifierItems, resources, templateType]); // eslint-disable-line react-hooks/exhaustive-deps

  return <div className="page-shell">
    {holder}
    <PageHeading title="商品配置中心" description="覆盖分类、套餐、加料、单位、标签、备注和可复用模板，并为点单小程序提供选项数据" extra={<Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button>} />
    <div className="catalog-overview">
      <Card bordered={false}><Typography.Text type="secondary"><AppstoreOutlined /> 分类</Typography.Text><strong>{categories.length}</strong></Card>
      <Card bordered={false}><Typography.Text type="secondary"><TagsOutlined /> 扩展字典</Typography.Text><strong>{resources.length}</strong></Card>
      <Card bordered={false}><Typography.Text type="secondary">加料商品</Typography.Text><strong>{modifierItems.length}</strong></Card>
      <Card bordered={false}><Typography.Text type="secondary">加料组</Typography.Text><strong>{modifierGroups.length}</strong></Card>
    </div>
    <Card bordered={false} className="content-card catalog-tabs"><Tabs items={tabs} /></Card>

    <Modal title={`${editingCategory ? '编辑' : '新增'}分类`} open={categoryOpen} onCancel={() => setCategoryOpen(false)} onOk={() => void saveCategory()}>
      <Form form={categoryForm} layout="vertical"><Form.Item label="分类名称" name="name" rules={[{ required: true }]}><Input /></Form.Item><Space align="start"><Form.Item label="排序" name="sort_order"><InputNumber min={0} /></Form.Item><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Space></Form>
    </Modal>
    <Modal title={`${editingItem ? '编辑' : '新增'}加料`} open={itemOpen} onCancel={() => setItemOpen(false)} onOk={() => void saveItem()}>
      <Form form={itemForm} layout="vertical"><Form.Item label="名称" name="name" rules={[{ required: true }]}><Input /></Form.Item><Form.Item label="加料图片" name="image_url"><ImagePickerField alt="加料图片" hint="将在商品规格与加料选择中展示" onOpenLibrary={() => setItemLibraryOpen(true)} /></Form.Item><Space align="start"><Form.Item label="默认加价" name="price" rules={[{ required: true }]}><InputNumber min={0} precision={2} prefix="¥" /></Form.Item><Form.Item label="排序" name="sort_order"><InputNumber min={0} /></Form.Item><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Space></Form>
    </Modal>
    <Modal title={`${editingGroup ? '编辑' : '新增'}加料组`} open={groupOpen} onCancel={() => setGroupOpen(false)} onOk={() => void saveGroup()}>
      <Form form={groupForm} layout="vertical"><Form.Item label="组名" name="name" rules={[{ required: true }]}><Input placeholder="如：加份小料" /></Form.Item><Space align="start"><Form.Item label="最少选择" name="min_select"><InputNumber min={0} /></Form.Item><Form.Item label="最多选择" name="max_select"><InputNumber min={1} /></Form.Item><Form.Item label="排序" name="sort_order"><InputNumber min={0} /></Form.Item><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Space><Form.Item label="包含加料" name="item_ids" rules={[{ required: true, message: '请至少选择一个加料' }]}><Select mode="multiple" options={modifierItems.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: `${item.name}（+¥${(item.price_cents / 100).toFixed(2)}）` }))} /></Form.Item></Form>
    </Modal>
    <MediaLibraryModal open={itemLibraryOpen} title="选择加料图片" onCancel={() => setItemLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) itemForm.setFieldValue('image_url', selected[0].url); setItemLibraryOpen(false); }} />
  </div>;
}
