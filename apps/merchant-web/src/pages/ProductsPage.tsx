import {
  AppstoreAddOutlined,
  DeleteOutlined,
  EditOutlined,
  InboxOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import {
  Avatar,
  Button,
  Card,
  Col,
  Divider,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import type { Category, Product, Sku } from '../types';
import { yuan } from '../utils/format';

interface ProductFormValues {
  name: string;
  categoryId: string | number;
  image?: string;
  description?: string;
  enabled: boolean;
  autoRestock: boolean;
  dailyStock?: number;
  skus: Sku[];
  optionGroups: Array<{
    name: string;
    selectionMode: 'SINGLE' | 'MULTIPLE';
    minSelect: number;
    maxSelect: number;
    values: Array<{ name: string; price: number; isDefault?: boolean }>;
  }>;
  modifierGroupIds: number[];
  resourceIds: number[];
}

interface ModifierGroupOption {
  id: number;
  name: string;
  min_select: number;
  max_select: number;
  status: string;
  items: Array<{ modifier_item_id: number; name: string; price_cents: number }>;
}

interface CatalogResourceOption {
  id: number;
  resource_type: string;
  name: string;
  status: string;
}

interface ProductConfiguration {
  option_groups: Array<{
    name: string;
    selection_mode: 'SINGLE' | 'MULTIPLE';
    min_select: number;
    max_select: number;
    values: Array<{ name: string; price_delta_cents: number; is_default: boolean }>;
  }>;
  modifier_groups: ModifierGroupOption[];
  resource_ids: number[];
}

function normalizeCategory(value: Category): Category {
  const raw = value as unknown as Record<string, unknown>;
  return {
    ...value,
    sort: Number(value.sort ?? raw.sort_order ?? 0),
    enabled: value.enabled ?? String(raw.status ?? 'ACTIVE') === 'ACTIVE',
  };
}

function normalizeProduct(value: Product): Product {
  const raw = value as unknown as Record<string, unknown>;
  const rawSkus = (raw.skus ?? []) as Array<Record<string, unknown>>;
  const skus: Sku[] = rawSkus.map((sku) => ({
    id: sku.id as string | number,
    name: String(sku.name ?? '默认规格'),
    price: sku.price_cents !== undefined ? Number(sku.price_cents) / 100 : Number(sku.price ?? 0),
    stock: Number(sku.stock ?? 0),
    expectedStock: Number(sku.stock ?? 0),
    originalStock: Number(sku.refill_stock ?? sku.stock ?? 0),
    attributes: (sku.attributes ?? {}) as Record<string, string>,
  }));
  return {
    ...value,
    categoryId: value.categoryId ?? (raw.category_id as string | number),
    image: value.image ?? String(raw.image_url ?? ''),
    description: value.description ?? String(raw.description ?? ''),
    enabled: value.enabled ?? String(raw.status ?? 'ACTIVE') === 'ACTIVE',
    // The database shape is retained for forward compatibility, but no daily
    // idempotent refill job exists yet. Never present a stored flag as active.
    autoRestock: false,
    dailyStock: value.dailyStock ?? Number(rawSkus[0]?.refill_stock ?? 0),
    skus,
    price: skus.length ? Math.min(...skus.map((sku) => sku.price)) : 0,
    stock: skus.reduce((sum, sku) => sum + sku.stock, 0),
  };
}

function productPayload(values: ProductFormValues | Product, enabled = values.enabled) {
  return {
    category_id: Number(values.categoryId),
    name: values.name,
    description: values.description ?? '',
    image_url: values.image ?? '',
    sort_order: 0,
    status: enabled ? 'ACTIVE' : 'DISABLED',
    skus: values.skus.map((sku) => ({
      id: Number(sku.id ?? 0),
      name: sku.name,
      attributes: sku.attributes ?? {},
      price_cents: Math.round(Number(sku.price) * 100),
      status: enabled ? 'ACTIVE' : 'DISABLED',
      stock: Number(sku.stock),
      expected_stock: sku.id ? Number(sku.expectedStock ?? sku.stock) : undefined,
      auto_sold_out: true,
      auto_refill: false,
      refill_stock: 0,
    })),
  };
}

export function ProductsPage() {
  const [categories, setCategories] = useState<Category[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [categoryId, setCategoryId] = useState<string>('ALL');
  const [keyword, setKeyword] = useState('');
  const [loading, setLoading] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Product | null>(null);
  const [saving, setSaving] = useState(false);
  const [configurationLoading, setConfigurationLoading] = useState(false);
  const [configurationReady, setConfigurationReady] = useState(true);
  const [configurationLoadError, setConfigurationLoadError] = useState('');
  const [categoryModal, setCategoryModal] = useState(false);
  const [modifierGroups, setModifierGroups] = useState<ModifierGroupOption[]>([]);
  const [catalogResources, setCatalogResources] = useState<CatalogResourceOption[]>([]);
  const [form] = Form.useForm<ProductFormValues>();
  const [categoryForm] = Form.useForm<{ name: string; sort?: number }>();
  const [messageApi, contextHolder] = message.useMessage();
  const configurationRequest = useRef(0);
  const saveRequest = useRef(0);
  const savingRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [categoryPayload, productResult, modifierResult, resourceResult] = await Promise.all([
        api.getList<Category>('/merchant/categories'),
        api.getList<Product>('/merchant/products', { keyword: keyword || undefined, page_size: 100 }),
        api.getList<ModifierGroupOption>('/merchant/modifier-groups'),
        api.getList<CatalogResourceOption>('/merchant/catalog-resources'),
      ]);
      setCategories(categoryPayload.items.map(normalizeCategory));
      setProducts(productResult.items.map(normalizeProduct));
      setModifierGroups(modifierResult.items);
      setCatalogResources(resourceResult.items);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi]);

  useEffect(() => { void load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const visibleProducts = useMemo(() => products.filter((product) => {
    const categoryMatches = categoryId === 'ALL' || String(product.categoryId) === categoryId;
    const keywordMatches = !keyword || product.name.toLowerCase().includes(keyword.toLowerCase());
    return categoryMatches && keywordMatches;
  }), [categoryId, keyword, products]);

  const openProduct = async (product?: Product) => {
    if (savingRef.current) return;
    const requestID = ++configurationRequest.current;
    setConfigurationLoading(Boolean(product));
    setConfigurationReady(!product);
    setConfigurationLoadError('');
    setEditing(product ?? null);
    form.resetFields();
    form.setFieldsValue(product ? {
      name: product.name,
      categoryId: product.categoryId,
      image: product.image,
      description: product.description,
      enabled: product.enabled,
      autoRestock: product.autoRestock ?? false,
      dailyStock: product.dailyStock,
      skus: product.skus?.length ? product.skus : [{ name: '默认规格', price: product.price, stock: product.stock }],
      optionGroups: [],
      modifierGroupIds: [],
      resourceIds: [],
    } : {
      enabled: true,
      autoRestock: false,
      skus: [{ name: '默认规格', price: 0, stock: 0 }],
      optionGroups: [],
      modifierGroupIds: [],
      resourceIds: [],
    });
    setDrawerOpen(true);
    if (product) {
      try {
        const config = await api.get<ProductConfiguration>(`/merchant/products/${product.id}/configuration`);
        if (requestID !== configurationRequest.current) return;
        form.setFieldsValue({
          optionGroups: config.option_groups.map((group) => ({
            name: group.name,
            selectionMode: group.selection_mode,
            minSelect: group.min_select,
            maxSelect: group.max_select,
            values: group.values.map((value) => ({ name: value.name, price: value.price_delta_cents / 100, isDefault: value.is_default })),
          })),
          modifierGroupIds: config.modifier_groups.map((group) => group.id),
          resourceIds: config.resource_ids,
        });
        setConfigurationReady(true);
      } catch (error) {
        if (requestID !== configurationRequest.current) return;
        const detail = errorMessage(error);
        setConfigurationLoadError(detail);
        setConfigurationReady(false);
        messageApi.error(`商品选项加载失败：${detail}`);
      } finally {
        if (requestID === configurationRequest.current) setConfigurationLoading(false);
      }
    }
  };

  const finalizeCloseProduct = () => {
    configurationRequest.current += 1;
    setConfigurationLoading(false);
    setConfigurationReady(true);
    setConfigurationLoadError('');
    setDrawerOpen(false);
  };

  const closeProduct = () => {
    if (savingRef.current) return;
    finalizeCloseProduct();
  };

  const saveProduct = async () => {
    if (savingRef.current) return;
    if (!configurationReady) {
      messageApi.error('点单配置尚未成功载入，请重试加载后再保存');
      return;
    }
    const values = await form.validateFields();
    const requestID = ++saveRequest.current;
    const targetProduct = editing;
    savingRef.current = true;
    setSaving(true);
    const payload = productPayload(values);
    try {
      const saved = targetProduct
        ? await api.put<Product>(`/merchant/products/${targetProduct.id}`, payload)
        : await api.post<Product>('/merchant/products', payload);
      if (requestID !== saveRequest.current) return;
      const normalized = normalizeProduct(saved);
      setEditing(normalized);
      setProducts((current) => targetProduct
        ? current.map((item) => item.id === targetProduct.id ? normalized : item)
        : [normalized, ...current]);
      try {
        await api.put(`/merchant/products/${normalized.id}/configuration`, {
        option_groups: (values.optionGroups || []).map((group, groupIndex) => ({
          name: group.name,
          kind: 'ATTRIBUTE',
          selection_mode: group.selectionMode,
          min_select: Number(group.minSelect || 0),
          max_select: Number(group.maxSelect || 1),
          sort_order: groupIndex,
          status: 'ACTIVE',
          values: (group.values || []).map((value, valueIndex) => ({
            name: value.name,
            price_delta_cents: Math.round(Number(value.price || 0) * 100),
            is_default: Boolean(value.isDefault),
            sort_order: valueIndex,
            status: 'ACTIVE',
          })),
        })),
        modifier_group_ids: values.modifierGroupIds || [],
          resource_ids: values.resourceIds || [],
        });
        if (requestID !== saveRequest.current) return;
      } catch (configurationError) {
        messageApi.error(`商品基础资料已保存，但点单选项保存失败：${errorMessage(configurationError)}。请修正后再次保存，不会重复创建商品。`);
        return;
      }
      finalizeCloseProduct();
      messageApi.success(targetProduct ? '商品已更新' : '商品已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  const toggleProduct = async (product: Product, enabled: boolean) => {
    try {
      const updated = normalizeProduct(await api.put<Product>(`/merchant/products/${product.id}`, productPayload(product, enabled)));
      setProducts((items) => items.map((item) => item.id === product.id ? updated : item));
      messageApi.success(enabled ? '商品已上架' : '商品已下架');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const deleteProduct = async (product: Product) => {
    try {
      await api.delete(`/merchant/products/${product.id}`);
      setProducts((items) => items.filter((item) => item.id !== product.id));
      messageApi.success('商品已删除');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const saveCategory = async () => {
    const values = await categoryForm.validateFields();
    setSaving(true);
    try {
      const saved = normalizeCategory(await api.post<Category>('/merchant/categories', { name: values.name, sort_order: values.sort ?? 0, status: 'ACTIVE' }));
      setCategories((items) => [...items, saved]);
      categoryForm.resetFields();
      setCategoryModal(false);
      messageApi.success('分类已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title="商品与库存"
        description="管理商品、规格库存、点单属性、加料组以及商品扩展标签"
        extra={<Space><Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openProduct()}>新增商品</Button></Space>}
      />
      <Row gutter={[16, 16]} align="stretch">
        <Col xs={24} lg={6} xl={5}>
          <Card
            bordered={false}
            className="content-card category-card"
            title="商品分类"
            extra={<Tooltip title="新增分类"><Button type="text" size="small" icon={<PlusOutlined />} onClick={() => setCategoryModal(true)} /></Tooltip>}
          >
            <List
              dataSource={[{ id: 'ALL', name: '全部商品', productCount: products.length }, ...categories]}
              renderItem={(category) => (
                <List.Item
                  className={`category-item ${categoryId === String(category.id) ? 'active' : ''}`}
                  onClick={() => setCategoryId(String(category.id))}
                  extra={<span>{category.productCount ?? products.filter((product) => String(product.categoryId) === String(category.id)).length}</span>}
                >
                  <Space><AppstoreAddOutlined /><span>{category.name}</span></Space>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col xs={24} lg={18} xl={19}>
          <Card bordered={false} className="content-card table-card">
            <div className="table-toolbar">
              <Input
                allowClear
                prefix={<SearchOutlined />}
                placeholder="搜索商品名称"
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                style={{ maxWidth: 360 }}
              />
              <Typography.Text type="secondary">共 {visibleProducts.length} 个商品</Typography.Text>
            </div>
            <Table<Product>
              rowKey="id"
              loading={loading}
              dataSource={visibleProducts}
              pagination={{ pageSize: 12, showTotal: (total) => `共 ${total} 个` }}
              scroll={{ x: 920 }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有商品，先创建一个吧" /> }}
              columns={[
                {
                  title: '商品', key: 'product', width: 260,
                  render: (_, product) => (
                    <Space>
                      <Avatar shape="square" size={52} src={product.image} icon={<InboxOutlined />} />
                      <div><Typography.Text strong>{product.name}</Typography.Text><div><Typography.Text type="secondary">{product.categoryName || categories.find((item) => String(item.id) === String(product.categoryId))?.name || '未分类'}</Typography.Text></div></div>
                    </Space>
                  ),
                },
                { title: '价格', dataIndex: 'price', width: 120, render: (value, product) => <strong>{yuan(value ?? product.skus?.[0]?.price)}</strong> },
                { title: '规格', key: 'skus', width: 100, render: (_, product) => `${product.skus?.length || 1} 个` },
                {
                  title: '库存', dataIndex: 'stock', width: 130,
                  render: (value: number) => <Space><strong className={value <= 5 ? 'stock-low' : ''}>{value}</strong>{value <= 0 ? <Tag color="error">售罄</Tag> : null}</Space>,
                },
                { title: '在售', dataIndex: 'enabled', width: 90, render: (value: boolean, product) => <Switch checked={value} onChange={(checked) => void toggleProduct(product, checked)} /> },
                {
                  title: '操作', key: 'action', width: 130, fixed: 'right',
                  render: (_, product) => <Space><Button type="link" icon={<EditOutlined />} onClick={() => openProduct(product)}>编辑</Button><Popconfirm title="确认删除该商品？" onConfirm={() => void deleteProduct(product)}><Button type="link" danger icon={<DeleteOutlined />} /></Popconfirm></Space>,
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      <Drawer
        title={editing ? '编辑商品' : '新增商品'}
        width={860}
        open={drawerOpen}
        onClose={closeProduct}
        maskClosable={!saving}
        keyboard={!saving}
        closable={!saving}
        extra={<Space><Button disabled={saving} onClick={closeProduct}>取消</Button><Button type="primary" loading={saving || configurationLoading} disabled={configurationLoading || !configurationReady} onClick={() => void saveProduct()}>{configurationLoading ? '正在加载点单配置' : '保存商品'}</Button></Space>}
      >
        {configurationLoadError ? (
          <Card size="small" className="configuration-load-error">
            <Space direction="vertical" size={8}>
              <Typography.Text type="danger">点单属性、加料和扩展绑定加载失败。为防止误清空原配置，当前禁止保存。</Typography.Text>
              <Space>
                <Button size="small" icon={<ReloadOutlined />} onClick={() => editing && void openProduct(editing)}>重新加载</Button>
                <Typography.Text type="secondary">{configurationLoadError}</Typography.Text>
              </Space>
            </Space>
          </Card>
        ) : null}
        <Form<ProductFormValues> form={form} layout="vertical" requiredMark="optional">
          <Row gutter={16}>
            <Col span={14}><Form.Item label="商品名称" name="name" rules={[{ required: true, message: '请输入商品名称' }]}><Input placeholder="例如：燕麦拿铁" /></Form.Item></Col>
            <Col span={10}><Form.Item label="商品分类" name="categoryId" rules={[{ required: true, message: '请选择分类' }]}><Select options={categories.map((item) => ({ label: item.name, value: item.id }))} placeholder="选择分类" /></Form.Item></Col>
          </Row>
          <Form.Item label="商品图片 URL" name="image"><Input placeholder="可填写对象存储中的图片地址" /></Form.Item>
          <Form.Item label="商品描述" name="description"><Input.TextArea rows={3} maxLength={200} showCount placeholder="介绍口味、原料或推荐理由" /></Form.Item>
          <Row gutter={16}>
            <Col span={12}><Form.Item label="立即上架" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col>
            <Col span={12}>
              <Form.Item
                label="每日营业自动填满库存（预留）"
                name="autoRestock"
                valuePropName="checked"
                extra="当前版本尚未启用幂等日切任务，避免误以为库存会自动恢复。"
              >
                <Switch disabled />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.autoRestock !== current.autoRestock}>
            {({ getFieldValue }) => getFieldValue('autoRestock') ? <Form.Item label="每日初始库存" name="dailyStock" rules={[{ required: true, message: '请输入每日库存' }]}><InputNumber min={0} precision={0} addonAfter="份" style={{ width: 220 }} /></Form.Item> : null}
          </Form.Item>
          <Divider orientation="left">规格与库存</Divider>
          <Form.List name="skus">
            {(fields, { add, remove }) => (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {fields.map((field, index) => (
                  <Card size="small" key={field.key} title={`规格 ${index + 1}`} extra={fields.length > 1 && <Button type="text" danger icon={<DeleteOutlined />} onClick={() => remove(field.name)} />}>
                    <Form.Item name={[field.name, 'expectedStock']} hidden><InputNumber /></Form.Item>
                    <Row gutter={12}>
                      <Col span={10}><Form.Item label="规格名称" name={[field.name, 'name']} rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="如：大杯 / 冰" /></Form.Item></Col>
                      <Col span={7}><Form.Item label="售价" name={[field.name, 'price']} rules={[{ required: true, message: '请输入售价' }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col>
                      <Col span={7}><Form.Item label="库存" name={[field.name, 'stock']} rules={[{ required: true, message: '请输入库存' }]}><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col>
                    </Row>
                  </Card>
                ))}
                <Button type="dashed" block icon={<PlusOutlined />} onClick={() => add({ name: '', price: 0, stock: 0 })}>增加规格</Button>
              </Space>
            )}
          </Form.List>
          <Divider orientation="left">点单属性</Divider>
          <Typography.Paragraph type="secondary">用于甜度、温度、是否去冰等不决定库存的点单选项；小程序会按必选和多选规则展示，并由服务端校验加价。</Typography.Paragraph>
          <Form.List name="optionGroups">
            {(groupFields, { add: addGroup, remove: removeGroup }) => (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {groupFields.map((groupField, groupIndex) => (
                  <Card size="small" key={groupField.key} title={`属性组 ${groupIndex + 1}`} extra={<Button type="text" danger icon={<DeleteOutlined />} onClick={() => removeGroup(groupField.name)} />}>
                    <Row gutter={12}>
                      <Col span={8}><Form.Item label="属性名称" name={[groupField.name, 'name']} rules={[{ required: true }]}><Input placeholder="如：甜度" /></Form.Item></Col>
                      <Col span={6}><Form.Item label="选择方式" name={[groupField.name, 'selectionMode']} rules={[{ required: true }]}><Select options={[{ value: 'SINGLE', label: '单选' }, { value: 'MULTIPLE', label: '多选' }]} /></Form.Item></Col>
                      <Col span={5}><Form.Item label="最少选" name={[groupField.name, 'minSelect']} rules={[{ required: true }]}><InputNumber min={0} style={{ width: '100%' }} /></Form.Item></Col>
                      <Col span={5}><Form.Item label="最多选" name={[groupField.name, 'maxSelect']} rules={[{ required: true }]}><InputNumber min={1} style={{ width: '100%' }} /></Form.Item></Col>
                    </Row>
                    <Form.List name={[groupField.name, 'values']}>
                      {(valueFields, { add: addValue, remove: removeValue }) => <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        {valueFields.map((valueField) => <Row gutter={10} key={valueField.key} align="middle">
                          <Col span={11}><Form.Item name={[valueField.name, 'name']} rules={[{ required: true, message: '请输入选项名称' }]} noStyle><Input placeholder="选项，如：无糖" /></Form.Item></Col>
                          <Col span={8}><Form.Item name={[valueField.name, 'price']} noStyle><InputNumber min={0} precision={2} prefix="+¥" style={{ width: '100%' }} /></Form.Item></Col>
                          <Col span={3}><Form.Item name={[valueField.name, 'isDefault']} valuePropName="checked" noStyle><Switch checkedChildren="默认" /></Form.Item></Col>
                          <Col span={2}><Button type="text" danger icon={<DeleteOutlined />} onClick={() => removeValue(valueField.name)} /></Col>
                        </Row>)}
                        <Button type="dashed" block onClick={() => addValue({ name: '', price: 0, isDefault: false })}>增加属性值</Button>
                      </Space>}
                    </Form.List>
                  </Card>
                ))}
                <Button type="dashed" block icon={<PlusOutlined />} onClick={() => addGroup({ name: '', selectionMode: 'SINGLE', minSelect: 1, maxSelect: 1, values: [{ name: '', price: 0 }] })}>增加点单属性组</Button>
              </Space>
            )}
          </Form.List>
          <Divider orientation="left">加料与商品扩展</Divider>
          <Form.Item label="绑定加料组" name="modifierGroupIds" extra="加料组在“商品配置中心”统一维护，绑定后会出现在小程序选项弹层。">
            <Select mode="multiple" allowClear placeholder="选择可用加料组" options={modifierGroups.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: `${item.name}（${item.min_select}–${item.max_select}项）` }))} />
          </Form.Item>
          <Form.Item label="单位、标签、备注与打印标签" name="resourceIds" extra="V1 保存商品与配置资料的绑定关系；运营筛选、快捷备注和真实打印路由将在对应能力接入后启用。">
            <Select mode="multiple" allowClear placeholder="选择商品扩展配置" options={catalogResources.filter((item) => item.status === 'ACTIVE' && !['PACKAGE', 'TEMP_PRODUCT'].includes(item.resource_type)).map((item) => ({ value: item.id, label: `${item.name} · ${item.resource_type}` }))} />
          </Form.Item>
        </Form>
      </Drawer>

      <Modal title="新增商品分类" open={categoryModal} onCancel={() => setCategoryModal(false)} onOk={() => void saveCategory()} confirmLoading={saving} okText="创建分类">
        <Form form={categoryForm} layout="vertical" initialValues={{ sort: categories.length + 1 }}>
          <Form.Item label="分类名称" name="name" rules={[{ required: true, message: '请输入分类名称' }]}><Input placeholder="例如：咖啡、气泡水" /></Form.Item>
          <Form.Item label="排序" name="sort"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
