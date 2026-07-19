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
import { useCallback, useEffect, useMemo, useState } from 'react';
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
    originalStock: Number(sku.refill_stock ?? sku.stock ?? 0),
    attributes: (sku.attributes ?? {}) as Record<string, string>,
  }));
  return {
    ...value,
    categoryId: value.categoryId ?? (raw.category_id as string | number),
    image: value.image ?? String(raw.image_url ?? ''),
    description: value.description ?? String(raw.description ?? ''),
    enabled: value.enabled ?? String(raw.status ?? 'ACTIVE') === 'ACTIVE',
    autoRestock: value.autoRestock ?? rawSkus.some((sku) => Boolean(sku.auto_refill)),
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
      auto_sold_out: true,
      auto_refill: Boolean(values.autoRestock),
      refill_stock: values.autoRestock ? Number(values.dailyStock ?? sku.originalStock ?? sku.stock) : 0,
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
  const [categoryModal, setCategoryModal] = useState(false);
  const [form] = Form.useForm<ProductFormValues>();
  const [categoryForm] = Form.useForm<{ name: string; sort?: number }>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [categoryPayload, productResult] = await Promise.all([
        api.getList<Category>('/merchant/categories'),
        api.getList<Product>('/merchant/products', { keyword: keyword || undefined, page_size: 100 }),
      ]);
      setCategories(categoryPayload.items.map(normalizeCategory));
      setProducts(productResult.items.map(normalizeProduct));
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

  const openProduct = (product?: Product) => {
    setEditing(product ?? null);
    form.setFieldsValue(product ? {
      name: product.name,
      categoryId: product.categoryId,
      image: product.image,
      description: product.description,
      enabled: product.enabled,
      autoRestock: product.autoRestock ?? false,
      dailyStock: product.dailyStock,
      skus: product.skus?.length ? product.skus : [{ name: '默认规格', price: product.price, stock: product.stock }],
    } : {
      enabled: true,
      autoRestock: false,
      skus: [{ name: '默认规格', price: 0, stock: 0 }],
    });
    setDrawerOpen(true);
  };

  const saveProduct = async () => {
    const values = await form.validateFields();
    setSaving(true);
    const payload = productPayload(values);
    try {
      const saved = editing
        ? await api.put<Product>(`/merchant/products/${editing.id}`, payload)
        : await api.post<Product>('/merchant/products', payload);
      const normalized = normalizeProduct(saved);
      setProducts((current) => editing
        ? current.map((item) => item.id === editing.id ? normalized : item)
        : [normalized, ...current]);
      setDrawerOpen(false);
      messageApi.success(editing ? '商品已更新' : '商品已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
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
        description="管理分类、规格、价格和每日可售库存"
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
                  render: (value: number, product) => <Space><strong className={value <= 5 ? 'stock-low' : ''}>{value}</strong>{value <= 0 ? <Tag color="error">售罄</Tag> : product.autoRestock ? <Tag color="blue">每日填满</Tag> : null}</Space>,
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
        width={720}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={<Space><Button onClick={() => setDrawerOpen(false)}>取消</Button><Button type="primary" loading={saving} onClick={() => void saveProduct()}>保存商品</Button></Space>}
      >
        <Form<ProductFormValues> form={form} layout="vertical" requiredMark="optional">
          <Row gutter={16}>
            <Col span={14}><Form.Item label="商品名称" name="name" rules={[{ required: true, message: '请输入商品名称' }]}><Input placeholder="例如：燕麦拿铁" /></Form.Item></Col>
            <Col span={10}><Form.Item label="商品分类" name="categoryId" rules={[{ required: true, message: '请选择分类' }]}><Select options={categories.map((item) => ({ label: item.name, value: item.id }))} placeholder="选择分类" /></Form.Item></Col>
          </Row>
          <Form.Item label="商品图片 URL" name="image"><Input placeholder="可填写对象存储中的图片地址" /></Form.Item>
          <Form.Item label="商品描述" name="description"><Input.TextArea rows={3} maxLength={200} showCount placeholder="介绍口味、原料或推荐理由" /></Form.Item>
          <Row gutter={16}>
            <Col span={12}><Form.Item label="立即上架" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col>
            <Col span={12}><Form.Item label="每日营业自动填满库存" name="autoRestock" valuePropName="checked"><Switch /></Form.Item></Col>
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
