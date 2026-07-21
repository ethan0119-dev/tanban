/* eslint-disable @next/next/no-img-element -- product media is rendered by the Vite merchant application */
import {
  AppstoreAddOutlined,
  BarChartOutlined,
  CheckCircleOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  InboxOutlined,
  MoreOutlined,
  PictureOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  StarFilled,
  StarOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  Avatar,
  Button,
  Card,
  Col,
  Divider,
  Drawer,
  Dropdown,
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
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import { PageHeading } from '../components/PageHeading';
import { merchantFeatureCopy } from '../features/availability/copy';
import type { MediaAsset } from '../features/media/model';
import type { Category, Product, ProductImage, Sku } from '../types';
import { dateTime, yuan } from '../utils/format';
import './products.css';

interface ProductFormValues {
  name: string;
  categoryId: string | number;
  image?: string;
  images: ProductImage[];
  description?: string;
  enabled: boolean;
  recommended: boolean;
  inStoreEnabled: boolean;
  deliveryEnabled: boolean;
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
  attributeGroupIds: number[];
  modifierGroupIds: number[];
  resourceIds: number[];
}

type ProductAction = 'ACTIVATE' | 'DEACTIVATE' | 'RECOMMEND' | 'UNRECOMMEND' | 'COPY' | 'SOLD_OUT' | 'RESTOCK_FULL';

interface ProductStatistics {
  productId: string | number;
  paidOrderCount: number;
  salesCount: number;
  grossSalesCents: number;
  from?: string;
  to?: string;
  metricScope?: string;
}

interface ModifierGroupOption {
  id: number;
  name: string;
  min_select: number;
  max_select: number;
  status: string;
  items: Array<{ modifier_item_id: number; name: string; price_cents: number }>;
}

interface AttributeGroupOption {
  id: number;
  name: string;
  selection_mode: 'SINGLE' | 'MULTIPLE';
  min_select: number;
  max_select: number;
  status: string;
  values: Array<{ id: number; name: string; price_delta_cents: number; status: string }>;
}

interface CatalogResourceOption {
  id: number;
  resource_type: string;
  name: string;
  status: string;
}

interface ProductConfiguration {
  option_groups: Array<{
    attribute_group_id?: number;
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
    inStoreEnabled: value.inStoreEnabled ?? Boolean(raw.in_store_enabled ?? true),
    deliveryEnabled: value.deliveryEnabled ?? Boolean(raw.delivery_enabled ?? false),
  };
}

export function normalizeProduct(value: Product): Product {
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
  const rawImages = (raw.images ?? []) as Array<Record<string, unknown>>;
  const images: ProductImage[] = rawImages.map((image, index) => ({
    id: image.id as string | number | undefined,
    mediaAssetId: (image.media_asset_id ?? image.mediaAssetId) as string | number | undefined,
    url: String(image.url ?? ''),
    isPrimary: Boolean(image.is_primary ?? image.isPrimary),
    sortOrder: Number(image.sort_order ?? image.sortOrder ?? index),
  })).filter((image) => image.url).sort((left, right) => Number(right.isPrimary) - Number(left.isPrimary) || left.sortOrder - right.sortOrder);
  const fallbackImage = value.image ?? String(raw.image_url ?? '');
  if (!images.length && fallbackImage) images.push({ url: fallbackImage, isPrimary: true, sortOrder: 0 });
  return {
    ...value,
    categoryId: value.categoryId ?? (raw.category_id as string | number),
    image: images.find((image) => image.isPrimary)?.url || images[0]?.url || fallbackImage,
    images,
    description: value.description ?? String(raw.description ?? ''),
    enabled: value.enabled ?? String(raw.status ?? 'ACTIVE') === 'ACTIVE',
    recommended: Boolean(value.recommended ?? raw.recommended),
    inStoreEnabled: value.inStoreEnabled ?? Boolean(raw.in_store_enabled ?? true),
    deliveryEnabled: value.deliveryEnabled ?? Boolean(raw.delivery_enabled ?? false),
    salesCount: Number(value.salesCount ?? raw.sales_count ?? 0),
    // The database shape is retained for forward compatibility, but no daily
    // idempotent refill job exists yet. Never present a stored flag as active.
    autoRestock: false,
    dailyStock: value.dailyStock ?? Number(rawSkus[0]?.refill_stock ?? 0),
    skus,
    price: skus.length ? Math.min(...skus.map((sku) => sku.price)) : 0,
    stock: skus.reduce((sum, sku) => sum + sku.stock, 0),
  };
}

export function productPayload(values: ProductFormValues | Product, enabled = values.enabled) {
  const images = (values.images || []).slice(0, 4).map((image, index) => ({
    media_asset_id: image.mediaAssetId ? Number(image.mediaAssetId) : undefined,
    url: image.url,
    is_primary: index === 0,
    sort_order: index,
  }));
  return {
    category_id: Number(values.categoryId),
    name: values.name,
    description: values.description ?? '',
    image_url: images[0]?.url ?? values.image ?? '',
    images,
    recommended: Boolean(values.recommended),
    in_store_enabled: values.inStoreEnabled !== false,
    delivery_enabled: Boolean(values.deliveryEnabled),
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
  const [editingCategory, setEditingCategory] = useState<Category | null>(null);
  const [imagePickerOpen, setImagePickerOpen] = useState(false);
  const [selectedProductIds, setSelectedProductIds] = useState<Array<string | number>>([]);
  const [actionLoading, setActionLoading] = useState<string>('');
  const [statisticsOpen, setStatisticsOpen] = useState(false);
  const [statisticsLoading, setStatisticsLoading] = useState(false);
  const [statisticsProduct, setStatisticsProduct] = useState<Product>();
  const [statistics, setStatistics] = useState<ProductStatistics>();
  const [restockProduct, setRestockProduct] = useState<Product>();
  const [restockStock, setRestockStock] = useState<number>(50);
  const [modifierGroups, setModifierGroups] = useState<ModifierGroupOption[]>([]);
  const [attributeGroups, setAttributeGroups] = useState<AttributeGroupOption[]>([]);
  const [catalogResources, setCatalogResources] = useState<CatalogResourceOption[]>([]);
  const [form] = Form.useForm<ProductFormValues>();
  const [categoryForm] = Form.useForm<{ name: string; sort?: number; inStoreEnabled: boolean; deliveryEnabled: boolean }>();
  const [messageApi, contextHolder] = message.useMessage();
  const configurationRequest = useRef(0);
  const saveRequest = useRef(0);
  const savingRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [categoryPayload, productResult, modifierResult, resourceResult, attributeResult] = await Promise.all([
        api.getList<Category>('/merchant/categories'),
        api.getList<Product>('/merchant/products', { keyword: keyword || undefined, page_size: 100 }),
        api.getList<ModifierGroupOption>('/merchant/modifier-groups'),
        api.getList<CatalogResourceOption>('/merchant/catalog-resources'),
        api.getList<AttributeGroupOption>('/merchant/attribute-groups'),
      ]);
      setCategories(categoryPayload.items.map(normalizeCategory));
      setProducts(productResult.items.map(normalizeProduct));
      setModifierGroups(modifierResult.items);
      setCatalogResources(resourceResult.items);
      setAttributeGroups(attributeResult.items);
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
      images: product.images || [],
      description: product.description,
      enabled: product.enabled,
      recommended: product.recommended ?? false,
      inStoreEnabled: product.inStoreEnabled !== false,
      deliveryEnabled: Boolean(product.deliveryEnabled),
      autoRestock: product.autoRestock ?? false,
      dailyStock: product.dailyStock,
      skus: product.skus?.length ? product.skus : [{ name: '默认规格', price: product.price, stock: product.stock }],
      optionGroups: [],
      attributeGroupIds: [],
      modifierGroupIds: [],
      resourceIds: [],
    } : {
      enabled: true,
      recommended: false,
      inStoreEnabled: true,
      deliveryEnabled: false,
      autoRestock: false,
      images: [],
      skus: [{ name: '默认规格', price: 0, stock: 0 }],
      optionGroups: [],
      attributeGroupIds: [],
      modifierGroupIds: [],
      resourceIds: [],
    });
    setDrawerOpen(true);
    if (product) {
      try {
        const config = await api.get<ProductConfiguration>(`/merchant/products/${product.id}/configuration`);
        if (requestID !== configurationRequest.current) return;
        form.setFieldsValue({
          attributeGroupIds: config.option_groups.flatMap((group) => group.attribute_group_id ? [group.attribute_group_id] : []),
          optionGroups: config.option_groups.filter((group) => !group.attribute_group_id).map((group) => ({
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
          attribute_group_ids: values.attributeGroupIds || [],
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
    const action: ProductAction = enabled ? 'ACTIVATE' : 'DEACTIVATE';
    setActionLoading(`${product.id}:${action}`);
    try {
      await api.post(`/merchant/products/${product.id}/actions`, { action });
      setProducts((items) => items.map((item) => item.id === product.id ? { ...item, enabled } : item));
      messageApi.success(enabled ? '商品已上架' : '商品已下架');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading('');
    }
  };

  const deleteProduct = async (product: Product) => {
    setActionLoading(`${product.id}:DELETE`);
    try {
      await api.delete(`/merchant/products/${product.id}`);
      setProducts((items) => items.filter((item) => item.id !== product.id));
      setSelectedProductIds((items) => items.filter((id) => String(id) !== String(product.id)));
      messageApi.success('商品已删除');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading('');
    }
  };

  const runProductAction = async (product: Product, action: ProductAction, stock?: number) => {
    setActionLoading(`${product.id}:${action}`);
    try {
      await api.post(`/merchant/products/${product.id}/actions`, { action, ...(action === 'RESTOCK_FULL' && stock !== undefined ? { stock } : {}) });
      if (action === 'RECOMMEND' || action === 'UNRECOMMEND') {
        setProducts((items) => items.map((item) => item.id === product.id ? { ...item, recommended: action === 'RECOMMEND' } : item));
      } else {
        await load();
      }
      const successMessages: Record<ProductAction, string> = {
        ACTIVATE: '商品已上架',
        DEACTIVATE: '商品已下架',
        RECOMMEND: '商品已推荐',
        UNRECOMMEND: '已取消推荐',
        COPY: '商品已复制为新的下架商品',
        SOLD_OUT: '商品库存已沽清',
        RESTOCK_FULL: '商品库存已置满',
      };
      messageApi.success(successMessages[action]);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading('');
    }
  };

  const runBatchAction = async (action: ProductAction | 'DELETE') => {
    const targets = products.filter((product) => selectedProductIds.some((id) => String(id) === String(product.id)));
    if (!targets.length) return;
    setActionLoading(`BATCH:${action}`);
    try {
      await Promise.all(targets.map((product) => action === 'DELETE'
        ? api.delete(`/merchant/products/${product.id}`)
        : api.post(`/merchant/products/${product.id}/actions`, { action })));
      setSelectedProductIds([]);
      await load();
      messageApi.success(`已处理 ${targets.length} 个商品`);
    } catch (error) {
      messageApi.error(`批量操作未全部完成，请刷新核对：${errorMessage(error)}`);
      await load();
    } finally {
      setActionLoading('');
    }
  };

  const openStatistics = async (product: Product) => {
    setStatisticsProduct(product);
    setStatistics(undefined);
    setStatisticsOpen(true);
    setStatisticsLoading(true);
    try {
      const payload = await api.get<Record<string, unknown>>(`/merchant/products/${product.id}/statistics`);
      setStatistics({
        productId: (payload.product_id ?? payload.productId ?? product.id) as string | number,
        paidOrderCount: Number(payload.paid_order_count ?? payload.paidOrderCount ?? 0),
        salesCount: Number(payload.sales_count ?? payload.salesCount ?? 0),
        grossSalesCents: Number(payload.gross_sales_cents ?? payload.grossSalesCents ?? 0),
        from: String(payload.from ?? ''),
        to: String(payload.to ?? ''),
        metricScope: String(payload.metric_scope ?? payload.metricScope ?? ''),
      });
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setStatisticsLoading(false);
    }
  };

  const saveCategory = async () => {
    const values = await categoryForm.validateFields();
    setSaving(true);
    try {
      const payload = { name: values.name, sort_order: values.sort ?? 0, status: editingCategory?.enabled === false ? 'DISABLED' : 'ACTIVE', in_store_enabled: values.inStoreEnabled !== false, delivery_enabled: false };
      const saved = normalizeCategory(editingCategory
        ? await api.put<Category>(`/merchant/categories/${editingCategory.id}`, payload)
        : await api.post<Category>('/merchant/categories', payload));
      setCategories((items) => editingCategory ? items.map((item) => item.id === editingCategory.id ? saved : item) : [...items, saved]);
      categoryForm.resetFields();
      setCategoryModal(false);
      setEditingCategory(null);
      messageApi.success(editingCategory ? '分类已更新' : '分类已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const openCategory = (category?: Category) => {
    setEditingCategory(category ?? null);
    categoryForm.setFieldsValue({
      name: category?.name ?? '',
      sort: category?.sort ?? categories.length + 1,
      inStoreEnabled: category?.inStoreEnabled !== false,
      deliveryEnabled: false,
    });
    setCategoryModal(true);
  };

  const toggleCategoryInStore = async (category: Category, enabled: boolean) => {
    try {
      const saved = normalizeCategory(await api.put<Category>(`/merchant/categories/${category.id}`, {
        name: category.name,
        sort_order: category.sort ?? 0,
        status: category.enabled === false ? 'DISABLED' : 'ACTIVE',
        in_store_enabled: enabled,
        delivery_enabled: false,
      }));
      setCategories((items) => items.map((item) => item.id === category.id ? saved : item));
      messageApi.success(enabled ? '分类已在店内显示' : '分类已从店内隐藏');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const toggleProductInStore = async (product: Product, enabled: boolean) => {
    setActionLoading(`${product.id}:CHANNEL`);
    try {
      const saved = normalizeProduct(await api.put<Product>(`/merchant/products/${product.id}`, productPayload({ ...product, inStoreEnabled: enabled, deliveryEnabled: false })));
      setProducts((items) => items.map((item) => item.id === product.id ? saved : item));
      messageApi.success(enabled ? '商品已在店内销售' : '商品已从店内隐藏');
    } catch (error) {
      messageApi.error(errorMessage(error));
      await load();
    } finally {
      setActionLoading('');
    }
  };

  const deliveryUnavailable = () => messageApi.info('外卖暂未开放，当前版本仅支持堂食和门店自取');

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title="商品管理"
        description="维护商品图片、规格库存、上下架与推荐状态，并查看成交统计"
        extra={<Space><Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openProduct()}>新增商品</Button></Space>}
      />
      <Row gutter={[16, 16]} align="stretch">
        <Col xs={24} lg={6} xl={5}>
          <Card
            bordered={false}
            className="content-card category-card"
            title="商品分类"
            extra={<Tooltip title="新增分类"><Button type="text" size="small" icon={<PlusOutlined />} onClick={() => openCategory()} /></Tooltip>}
          >
            <List
              dataSource={[{ id: 'ALL', name: '全部商品', productCount: products.length }, ...categories]}
              renderItem={(category) => (
                <List.Item
                  className={`category-item ${categoryId === String(category.id) ? 'active' : ''}`}
                  onClick={() => setCategoryId(String(category.id))}
                  extra={String(category.id) === 'ALL' ? <span>{category.productCount}</span> : <Space size={4} onClick={(event) => event.stopPropagation()}>
                    <Tooltip title="店内显示"><Switch size="small" checked={category.inStoreEnabled !== false} onChange={(checked) => void toggleCategoryInStore(category as Category, checked)} /></Tooltip>
                    <Tooltip title="外卖暂未开放"><Switch size="small" checked={false} onChange={(checked) => checked && deliveryUnavailable()} /></Tooltip>
                    <Button type="text" size="small" icon={<EditOutlined />} aria-label="编辑分类" onClick={() => openCategory(category as Category)} />
                  </Space>}
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
            {selectedProductIds.length > 0 && (
              <div className="product-batch-bar">
                <Typography.Text strong>已选择 {selectedProductIds.length} 个商品</Typography.Text>
                <Space wrap>
                  <Button size="small" loading={actionLoading === 'BATCH:ACTIVATE'} onClick={() => void runBatchAction('ACTIVATE')}>批量上架</Button>
                  <Button size="small" loading={actionLoading === 'BATCH:DEACTIVATE'} onClick={() => void runBatchAction('DEACTIVATE')}>批量下架</Button>
                  <Button size="small" icon={<StarOutlined />} onClick={() => void runBatchAction('RECOMMEND')}>批量推荐</Button>
                  <Button size="small" onClick={() => void runBatchAction('UNRECOMMEND')}>取消推荐</Button>
                  <Popconfirm title={`删除所选 ${selectedProductIds.length} 个商品？`} description="删除后不可恢复。" onConfirm={() => void runBatchAction('DELETE')}><Button size="small" danger>批量删除</Button></Popconfirm>
                  <Button size="small" type="text" onClick={() => setSelectedProductIds([])}>取消选择</Button>
                </Space>
              </div>
            )}
            <Table<Product>
              rowKey="id"
              rowSelection={{ selectedRowKeys: selectedProductIds, onChange: (keys) => setSelectedProductIds(keys as Array<string | number>) }}
              loading={loading}
              dataSource={visibleProducts}
              pagination={{ pageSize: 12, showTotal: (total) => `共 ${total} 个` }}
              scroll={{ x: 1370 }}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有商品，先创建一个吧" /> }}
              columns={[
                {
                  title: '商品', key: 'product', width: 300,
                  render: (_, product) => (
                    <Space>
                      <Avatar shape="square" size={58} src={product.image} icon={<InboxOutlined />} />
                      <div>
                        <Space size={6}><Typography.Text strong>{product.name}</Typography.Text>{product.recommended && <Tag color="volcano" icon={<StarFilled />}>推荐</Tag>}</Space>
                        <div><Typography.Text type="secondary">{product.categoryName || categories.find((item) => String(item.id) === String(product.categoryId))?.name || '未分类'} · {product.images?.length || 0} 张图</Typography.Text></div>
                        <div><Typography.Text type="secondary">销量 {product.salesCount || 0}</Typography.Text></div>
                      </div>
                    </Space>
                  ),
                },
                { title: '价格', dataIndex: 'price', width: 120, render: (value, product) => <strong>{yuan(value ?? product.skus?.[0]?.price)}</strong> },
                { title: '规格', key: 'skus', width: 100, render: (_, product) => `${product.skus?.length || 1} 个` },
                {
                  title: '库存', dataIndex: 'stock', width: 130,
                  render: (value: number) => <Space><strong className={value <= 5 ? 'stock-low' : ''}>{value}</strong>{value <= 0 ? <Tag color="error">售罄</Tag> : null}</Space>,
                },
                { title: '在售', dataIndex: 'enabled', width: 90, render: (value: boolean, product) => <Switch loading={actionLoading === `${product.id}:${value ? 'DEACTIVATE' : 'ACTIVATE'}`} checked={value} onChange={(checked) => void toggleProduct(product, checked)} /> },
                {
                  title: '销售渠道', key: 'channels', width: 170,
                  render: (_, product) => <Space direction="vertical" size={3}>
                    <Space size={6}><Typography.Text type="secondary">店内</Typography.Text><Switch size="small" loading={actionLoading === `${product.id}:CHANNEL`} checked={product.inStoreEnabled !== false} onChange={(checked) => void toggleProductInStore(product, checked)} /></Space>
                    <Space size={6}><Typography.Text type="secondary">外卖</Typography.Text><Switch size="small" checked={false} onChange={(checked) => checked && deliveryUnavailable()} /></Space>
                  </Space>,
                },
                {
                  title: '操作', key: 'action', width: 340, fixed: 'right',
                  render: (_, product) => (
                    <Space size={[2, 2]} wrap className="product-row-actions">
                      <Button type="link" icon={<EditOutlined />} onClick={() => openProduct(product)}>编辑</Button>
                      <Button type="link" icon={product.recommended ? <StarFilled /> : <StarOutlined />} loading={actionLoading === `${product.id}:${product.recommended ? 'UNRECOMMEND' : 'RECOMMEND'}`} onClick={() => void runProductAction(product, product.recommended ? 'UNRECOMMEND' : 'RECOMMEND')}>{product.recommended ? '取消推荐' : '推荐'}</Button>
                      <Button type="link" icon={<BarChartOutlined />} onClick={() => void openStatistics(product)}>商品统计</Button>
                      <Button type="link" icon={<CopyOutlined />} loading={actionLoading === `${product.id}:COPY`} onClick={() => void runProductAction(product, 'COPY')}>复制</Button>
                      <Dropdown
                        menu={{
                          items: [
                            { key: 'sold-out', label: '沽清库存', icon: <StopOutlined />, disabled: product.stock <= 0 },
                            { key: 'restock', label: '置满库存', icon: <CheckCircleOutlined /> },
                            { type: 'divider' },
                            { key: 'delete', label: '删除商品', icon: <DeleteOutlined />, danger: true },
                          ],
                          onClick: ({ key, domEvent }) => {
                            domEvent.stopPropagation();
                            if (key === 'sold-out') void runProductAction(product, 'SOLD_OUT');
                            if (key === 'restock') { setRestockProduct(product); setRestockStock(product.dailyStock || Math.max(50, ...product.skus.map((sku) => sku.stock))); }
                            if (key === 'delete') Modal.confirm({ title: `删除“${product.name}”？`, content: '删除后不可恢复，历史订单中的商品快照不受影响。', okButtonProps: { danger: true }, okText: '确认删除', onOk: () => deleteProduct(product) });
                          },
                        }}
                      ><Button type="link" icon={<MoreOutlined />}>更多</Button></Dropdown>
                    </Space>
                  ),
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
          <Form.Item name="recommended" hidden valuePropName="checked"><Switch /></Form.Item>
          <Form.Item
            label="商品图片"
            name="images"
            rules={[{ validator: (_, value: ProductImage[]) => !value?.length || (value.length <= 4 && value.filter((item) => item.isPrimary).length === 1) ? Promise.resolve() : Promise.reject(new Error('商品最多 4 张图片，并且需要 1 张主图')) }]}
            extra="最多 4 张：第 1 张为商品主图，其余为详情辅图；图片来自当前商户图片库，可在装修和营销中复用。"
          >
            <ProductImagesField onOpenLibrary={() => setImagePickerOpen(true)} />
          </Form.Item>
          <Form.Item label="商品描述" name="description"><Input.TextArea rows={3} maxLength={200} showCount placeholder="介绍口味、原料或推荐理由" /></Form.Item>
          <Row gutter={16}>
            <Col span={12}><Form.Item label="立即上架" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col>
            <Col span={12}>
              <Form.Item
                label="每日营业自动填满库存（暂未开放）"
                name="autoRestock"
                valuePropName="checked"
                extra={merchantFeatureCopy.PRODUCT_AUTO_RESTOCK.description}
              >
                <Switch disabled />
              </Form.Item>
            </Col>
          </Row>
          <Divider orientation="left">销售渠道</Divider>
          <Row gutter={16}>
            <Col span={12}><Form.Item label="店内" name="inStoreEnabled" valuePropName="checked" extra="同时用于堂食与门店自取"><Switch checkedChildren="显示" unCheckedChildren="隐藏" /></Form.Item></Col>
            <Col span={12}><Form.Item label="外卖（暂未开放）" name="deliveryEnabled" valuePropName="checked" extra="外卖订单与配送能力接入后开放"><Switch checked={false} onChange={(checked) => { if (checked) deliveryUnavailable(); form.setFieldValue('deliveryEnabled', false); }} /></Form.Item></Col>
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
          <Typography.Paragraph type="secondary">优先从属性库选择温度、甜度等通用选项；属性库后续调整会同步到所有关联商品。只有该商品独有的选项才需要在下方单独维护。</Typography.Paragraph>
          <Form.Item label="从属性库选择" name="attributeGroupIds" extra="属性定义在“商品配置中心 → 属性库”统一维护；选择顺序即小程序展示顺序。">
            <Select
              mode="multiple"
              allowClear
              placeholder="选择温度、甜度等属性"
              options={attributeGroups.filter((item) => item.status === 'ACTIVE').map((item) => ({
                value: item.id,
                label: `${item.name}（${item.selection_mode === 'SINGLE' ? '单选' : `${item.min_select}–${item.max_select}项`}：${item.values.filter((value) => value.status === 'ACTIVE').map((value) => value.name).join('、')}）`,
              }))}
            />
          </Form.Item>
          <Typography.Text strong>商品专属属性（可选）</Typography.Text>
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
                <Button type="dashed" block icon={<PlusOutlined />} onClick={() => addGroup({ name: '', selectionMode: 'SINGLE', minSelect: 1, maxSelect: 1, values: [{ name: '', price: 0 }] })}>增加商品专属属性组</Button>
              </Space>
            )}
          </Form.List>
          <Divider orientation="left">加料与商品扩展</Divider>
          <Form.Item label="绑定加料组" name="modifierGroupIds" extra="加料组在“商品配置中心”统一维护，绑定后会出现在小程序选项弹层。">
            <Select mode="multiple" allowClear placeholder="选择可用加料组" options={modifierGroups.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: `${item.name}（${item.min_select}–${item.max_select}项）` }))} />
          </Form.Item>
          <Form.Item label="单位、标签、备注与打印标签" name="resourceIds" extra={merchantFeatureCopy.CATALOG_TEMPLATE_APPLICATION.description}>
            <Select mode="multiple" allowClear placeholder="选择商品扩展配置" options={catalogResources.filter((item) => item.status === 'ACTIVE' && !['PACKAGE', 'TEMP_PRODUCT'].includes(item.resource_type)).map((item) => ({ value: item.id, label: `${item.name} · ${item.resource_type}` }))} />
          </Form.Item>
        </Form>
      </Drawer>

      <Modal title={editingCategory ? '编辑商品分类' : '新增商品分类'} open={categoryModal} onCancel={() => { setCategoryModal(false); setEditingCategory(null); }} onOk={() => void saveCategory()} confirmLoading={saving} okText={editingCategory ? '保存分类' : '创建分类'}>
        <Form form={categoryForm} layout="vertical" initialValues={{ sort: categories.length + 1, inStoreEnabled: true, deliveryEnabled: false }}>
          <Form.Item label="分类名称" name="name" rules={[{ required: true, message: '请输入分类名称' }]}><Input placeholder="例如：咖啡、气泡水" /></Form.Item>
          <Form.Item label="排序" name="sort"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
          <Row gutter={16}>
            <Col span={12}><Form.Item label="店内" name="inStoreEnabled" valuePropName="checked"><Switch checkedChildren="显示" unCheckedChildren="隐藏" /></Form.Item></Col>
            <Col span={12}><Form.Item label="外卖（暂未开放）" name="deliveryEnabled" valuePropName="checked"><Switch checked={false} onChange={(checked) => { if (checked) deliveryUnavailable(); categoryForm.setFieldValue('deliveryEnabled', false); }} /></Form.Item></Col>
          </Row>
        </Form>
      </Modal>

      <MediaLibraryModal
        open={imagePickerOpen}
        title="选择商品图片"
        multiple
        maxSelection={Math.max(1, 4 - ((form.getFieldValue('images') as ProductImage[] | undefined)?.length || 0))}
        excludeUrls={((form.getFieldValue('images') as ProductImage[] | undefined) || []).map((item) => item.url)}
        onCancel={() => setImagePickerOpen(false)}
        onConfirm={(assets) => {
          const current = (form.getFieldValue('images') as ProductImage[] | undefined) || [];
          const additions = assets.map((asset) => mediaAssetToProductImage(asset));
          const next = [...current, ...additions].slice(0, 4).map((image, index) => ({ ...image, isPrimary: index === 0, sortOrder: index }));
          form.setFieldValue('images', next);
          setImagePickerOpen(false);
        }}
      />

      <Modal
        title={restockProduct ? `置满库存 · ${restockProduct.name}` : '置满库存'}
        open={Boolean(restockProduct)}
        okText="确认置满"
        confirmLoading={Boolean(restockProduct && actionLoading === `${restockProduct.id}:RESTOCK_FULL`)}
        onCancel={() => setRestockProduct(undefined)}
        onOk={async () => {
          if (!restockProduct) return;
          await runProductAction(restockProduct, 'RESTOCK_FULL', restockStock);
          setRestockProduct(undefined);
        }}
      >
        <Typography.Paragraph type="secondary">将该商品下所有规格的可售库存统一设为指定数量。库存为 0 时可用它恢复销售。</Typography.Paragraph>
        <InputNumber min={0} max={999999} precision={0} addonAfter="每个规格" value={restockStock} onChange={(value) => setRestockStock(Number(value || 0))} style={{ width: '100%' }} />
      </Modal>

      <Modal title={`商品统计${statisticsProduct ? ` · ${statisticsProduct.name}` : ''}`} open={statisticsOpen} footer={<Button onClick={() => setStatisticsOpen(false)}>关闭</Button>} onCancel={() => setStatisticsOpen(false)}>
        <Card loading={statisticsLoading} bordered={false} className="product-statistics-card">
          {statistics ? <>
            <Row gutter={[12, 16]}>
              <Col span={8}><div className="product-statistic"><span>支付订单</span><strong>{statistics.paidOrderCount}</strong><small>笔</small></div></Col>
              <Col span={8}><div className="product-statistic"><span>销售数量</span><strong>{statistics.salesCount}</strong><small>份</small></div></Col>
              <Col span={8}><div className="product-statistic"><span>毛销售额</span><strong>{yuan(statistics.grossSalesCents / 100)}</strong></div></Col>
            </Row>
            <Divider />
            <Typography.Paragraph type="secondary">统计口径：已支付订单中的商品毛销售，不扣除后续退款。{statistics.metricScope ? ` · ${statistics.metricScope}` : ''}</Typography.Paragraph>
            {(statistics.from || statistics.to) && <Typography.Text type="secondary">区间：{formatDateTime(statistics.from)} 至 {formatDateTime(statistics.to)}</Typography.Text>}
          </> : !statisticsLoading ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可展示的统计数据" /> : null}
        </Card>
      </Modal>
    </div>
  );
}

function ProductImagesField({ value = [], onChange, onOpenLibrary }: { value?: ProductImage[]; onChange?: (images: ProductImage[]) => void; onOpenLibrary: () => void }) {
  const ordered = [...value].sort((left, right) => Number(right.isPrimary) - Number(left.isPrimary) || left.sortOrder - right.sortOrder);
  const emit = (images: ProductImage[]) => onChange?.(images.map((image, index) => ({ ...image, isPrimary: index === 0, sortOrder: index })));
  return (
    <div className="product-image-field">
      {ordered.map((image, index) => (
        <div className={`product-image-tile ${index === 0 ? 'primary' : ''}`} key={`${image.url}-${index}`}>
          <img src={image.url} alt={index === 0 ? '商品主图' : `商品辅图 ${index}`} />
          <span>{index === 0 ? '主图' : `辅图 ${index}`}</span>
          <div className="product-image-actions">
            {index > 0 && <Button size="small" type="text" onClick={() => emit([ordered[index], ...ordered.filter((_, itemIndex) => itemIndex !== index)])}>设为主图</Button>}
            <Button size="small" type="text" danger icon={<DeleteOutlined />} aria-label="移除图片" onClick={() => emit(ordered.filter((_, itemIndex) => itemIndex !== index))} />
          </div>
        </div>
      ))}
      {ordered.length < 4 && <button type="button" className="product-image-add" onClick={onOpenLibrary}><PictureOutlined /><strong>从图片库选择</strong><small>还可添加 {4 - ordered.length} 张</small></button>}
    </div>
  );
}

function mediaAssetToProductImage(asset: MediaAsset): ProductImage {
  return { mediaAssetId: asset.id, url: asset.url, isPrimary: false, sortOrder: 0 };
}

function formatDateTime(value?: string) {
  if (!value) return '不限';
  return dateTime(value);
}
