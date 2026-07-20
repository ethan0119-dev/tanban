import { AimOutlined, CloudUploadOutlined, DeleteOutlined, PictureOutlined } from '@ant-design/icons';
import { Alert, Button, Col, Empty, Form, Input, InputNumber, Modal, Row, Select, Space, Tag, Upload, Typography, message } from 'antd';
import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import type { DecorationActionType, HomeModuleConfig, ImageHotspot, MediaAsset } from '../../features/decoration/model';

const ACTION_OPTIONS: Array<{ value: DecorationActionType; label: string }> = [
  { value: 'NONE', label: '无动作' },
  { value: 'OPEN_MENU', label: '打开点单' },
  { value: 'OPEN_ORDERS', label: '打开订单' },
  { value: 'OPEN_PROFILE', label: '打开我的' },
  { value: 'CALL_PHONE', label: '拨打电话' },
];
const MAX_HOTSPOTS = 20;

interface HotspotImageEditorProps {
  module: HomeModuleConfig;
  assets: MediaAsset[];
  uploading: boolean;
  onChange: (patch: Partial<HomeModuleConfig>) => void;
  onUpload: (file: File) => Promise<MediaAsset>;
}

interface DrawState {
  startX: number;
  startY: number;
  x: number;
  y: number;
  width: number;
  height: number;
}

export function HotspotImageEditor({ module, assets, uploading, onChange, onUpload }: HotspotImageEditorProps) {
  const [drawMode, setDrawMode] = useState(false);
  const [drawing, setDrawing] = useState<DrawState | null>(null);
  const [selectedId, setSelectedId] = useState<string>('');
  const [imageUrlDraft, setImageUrlDraft] = useState(module.imageUrl ?? '');
  const [messageApi, contextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();
  const canvasRef = useRef<HTMLDivElement>(null);
  const hotspots = module.hotspots ?? [];
  const selected = hotspots.find((item) => item.id === selectedId) ?? null;
  const imageAssets = useMemo(() => assets.filter((item) => item.type === 'IMAGE' && item.url), [assets]);

  useEffect(() => {
    setImageUrlDraft(module.imageUrl ?? '');
  }, [module.imageUrl]);

  const applyImageChange = async (imageUrl: string, nextTitle?: string): Promise<boolean> => {
    const nextImageUrl = imageUrl.trim();
    const currentImageUrl = (module.imageUrl ?? '').trim();
    if (nextImageUrl === currentImageUrl) {
      if (nextTitle && nextTitle !== module.title) onChange({ title: nextTitle });
      setImageUrlDraft(nextImageUrl);
      return true;
    }

    if (hotspots.length > 0) {
      const confirmed = await new Promise<boolean>((resolve) => {
        modal.confirm({
          title: '更换图片并清空现有热区？',
          content: `当前 ${hotspots.length} 个热区是按原图位置计算的。更换图片后必须重新绘制，旧热区将全部清空。`,
          okText: '更换并清空',
          cancelText: '保留原图',
          okButtonProps: { danger: true },
          onOk: () => resolve(true),
          onCancel: () => resolve(false),
        });
      });
      if (!confirmed) {
        setImageUrlDraft(currentImageUrl);
        return false;
      }
    }

    onChange({ imageUrl: nextImageUrl, hotspots: [], ...(nextTitle ? { title: nextTitle } : {}) });
    setImageUrlDraft(nextImageUrl);
    setSelectedId('');
    setDrawing(null);
    setDrawMode(false);
    if (hotspots.length > 0) messageApi.info('图片已更换，原有热区已清空，请重新绘制');
    return true;
  };

  const patchHotspot = (id: string, patch: Partial<ImageHotspot>) => {
    onChange({ hotspots: hotspots.map((item) => item.id === id ? normalizeHotspot({ ...item, ...patch }) : item) });
  };

  const deleteHotspot = (id: string) => {
    onChange({ hotspots: hotspots.filter((item) => item.id !== id) });
    if (selectedId === id) setSelectedId('');
  };

  const pointFromEvent = (event: ReactPointerEvent<HTMLDivElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect?.width || !rect.height) return { x: 0, y: 0 };
    return {
      x: clamp(((event.clientX - rect.left) / rect.width) * 100, 0, 100),
      y: clamp(((event.clientY - rect.top) / rect.height) * 100, 0, 100),
    };
  };

  const startDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drawMode || !module.imageUrl || hotspots.length >= MAX_HOTSPOTS) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    const point = pointFromEvent(event);
    setDrawing({ startX: point.x, startY: point.y, x: point.x, y: point.y, width: 0, height: 0 });
  };

  const moveDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drawing) return;
    const point = pointFromEvent(event);
    setDrawing(rectFromPoints(drawing.startX, drawing.startY, point.x, point.y));
  };

  const finishDrawing = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drawing) return;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    if (drawing.width >= 2 && drawing.height >= 2) {
      const next: ImageHotspot = {
        id: `hotspot-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
        x: round(drawing.x),
        y: round(drawing.y),
        width: round(drawing.width),
        height: round(drawing.height),
        label: `热区 ${hotspots.length + 1}`,
        action: { type: 'OPEN_MENU' },
      };
      onChange({ hotspots: [...hotspots, next] });
      setSelectedId(next.id);
      setDrawMode(false);
    } else {
      messageApi.warning('热区太小，请在图片上拖出一个矩形区域');
    }
    setDrawing(null);
  };

  const uploadFile = async (file: File) => {
    try {
      const asset = await onUpload(file);
      const applied = await applyImageChange(asset.url, module.title || asset.name);
      messageApi.success(applied ? '图片已上传并应用，现在可以绘制热区' : '图片已上传到素材库，当前热区图未更换');
      return asset;
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '图片上传失败');
      throw error;
    }
  };

  return (
    <div className="hotspot-editor">
      {contextHolder}
      {modalContextHolder}
      <Alert
        showIcon
        type="info"
        message="上传一张完整首页图，在图上拖拽绘制可点击区域"
        description="热区仅可绑定系统提供的安全动作，不执行自定义脚本或任意外链。"
      />
      <Form layout="vertical">
        <Row gutter={12} align="bottom">
          <Col xs={24} lg={12}>
            <Form.Item label="从素材库选择">
              <Select
                allowClear
                showSearch
                value={module.imageUrl || undefined}
                placeholder="选择已上传图片"
                optionFilterProp="label"
                options={imageAssets.map((asset) => ({ value: asset.url, label: asset.name }))}
                onChange={(imageUrl) => void applyImageChange(imageUrl ?? '')}
              />
            </Form.Item>
          </Col>
          <Col xs={24} lg={12}>
            <Form.Item label="上传新图片">
              <Upload
                accept="image/jpeg,image/png,image/gif"
                maxCount={1}
                showUploadList={false}
                beforeUpload={(file) => {
                  if (file.size > 8 * 1024 * 1024) {
                    messageApi.error('图片不能超过 8MB');
                    return Upload.LIST_IGNORE;
                  }
                  return true;
                }}
                customRequest={({ file, onError, onSuccess }) => {
                  void uploadFile(file as File).then((asset) => onSuccess?.(asset)).catch((error: Error) => onError?.(error));
                }}
              >
                <Button block loading={uploading} icon={<CloudUploadOutlined />}>本地上传 JPG / PNG / GIF</Button>
              </Upload>
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label="图片 HTTPS 地址" extra="修改地址后点击应用；如已有热区，系统会先确认并清空旧坐标。">
          <Space.Compact block>
            <Input value={imageUrlDraft} prefix={<PictureOutlined />} placeholder="https://..." onChange={(event) => setImageUrlDraft(event.target.value)} onPressEnter={() => void applyImageChange(imageUrlDraft)} />
            <Button disabled={imageUrlDraft.trim() === (module.imageUrl ?? '').trim()} onClick={() => void applyImageChange(imageUrlDraft)}>应用地址</Button>
          </Space.Compact>
        </Form.Item>
        <Form.Item label="图片说明">
          <Input value={module.title} maxLength={80} showCount placeholder="例如：码农咖啡首页导航" onChange={(event) => onChange({ title: event.target.value })} />
        </Form.Item>
      </Form>

      <div className="hotspot-toolbar">
        <div><strong>热区编辑</strong><Typography.Text type="secondary">共 {hotspots.length} 个，最多 {MAX_HOTSPOTS} 个</Typography.Text></div>
        <Button type={drawMode ? 'primary' : 'default'} icon={<AimOutlined />} disabled={!module.imageUrl || hotspots.length >= MAX_HOTSPOTS} onClick={() => { setDrawMode((value) => !value); setDrawing(null); }}>
          {drawMode ? '取消绘制' : '绘制新热区'}
        </Button>
      </div>
      {hotspots.length === 0 && <Alert type="warning" showIcon message="当前图片还没有可点击区域" description="可以先作为普通图片保存；绘制热区并绑定动作后，顾客点击对应区域才会跳转或拨号。" />}

      {module.imageUrl ? (
        <div
          ref={canvasRef}
          className={`hotspot-canvas ${drawMode ? 'drawing' : ''}`}
          onPointerDown={startDrawing}
          onPointerMove={moveDrawing}
          onPointerUp={finishDrawing}
          onPointerCancel={() => setDrawing(null)}
        >
          {/* Native img keeps the overlay coordinate system identical to the rendered bitmap. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={module.imageUrl} alt={module.title || '热区图片'} draggable={false} />
          {hotspots.map((hotspot, index) => (
            <button
              key={hotspot.id}
              type="button"
              className={`hotspot-box ${hotspot.id === selectedId ? 'selected' : ''}`}
              style={hotspotStyle(hotspot)}
              aria-label={`编辑${hotspot.label}`}
              onPointerDown={(event) => event.stopPropagation()}
              onClick={() => setSelectedId(hotspot.id)}
            >
              <span>{index + 1}</span><small>{hotspot.label}</small>
            </button>
          ))}
          {drawing && <div className="hotspot-box drawing-box" style={hotspotStyle(drawing)}><span>+</span></div>}
        </div>
      ) : <Empty className="hotspot-empty" image={Empty.PRESENTED_IMAGE_SIMPLE} description="先上传、选择或填入一张 HTTPS 图片" />}

      {selected ? (
        <div className="hotspot-inspector">
          <div className="hotspot-inspector-heading"><Space><Tag color="blue">热区 {hotspots.findIndex((item) => item.id === selected.id) + 1}</Tag><strong>{selected.label}</strong></Space><Button danger type="text" icon={<DeleteOutlined />} onClick={() => deleteHotspot(selected.id)}>删除</Button></div>
          <Form layout="vertical">
            <Row gutter={12}>
              <Col xs={24} lg={12}><Form.Item label="热区名称"><Input value={selected.label} maxLength={30} onChange={(event) => patchHotspot(selected.id, { label: event.target.value })} /></Form.Item></Col>
              <Col xs={24} lg={12}><Form.Item label="点击动作"><Select value={selected.action.type} options={ACTION_OPTIONS} onChange={(type) => patchHotspot(selected.id, { action: type === 'CALL_PHONE' ? { type, phone: '' } : { type } })} /></Form.Item></Col>
            </Row>
            {selected.action.type === 'CALL_PHONE' && <Form.Item label="商家电话" help="顾客点击后调起微信拨号确认"><Input value={selected.action.phone} maxLength={20} placeholder="例如：18600000000" onChange={(event) => patchHotspot(selected.id, { action: { type: 'CALL_PHONE', phone: event.target.value.replace(/[^0-9+\- ]/g, '') } })} /></Form.Item>}
            <Row gutter={8}>
              {(['x', 'y', 'width', 'height'] as const).map((key) => <Col span={6} key={key}><Form.Item label={{ x: 'X', y: 'Y', width: '宽', height: '高' }[key]}><InputNumber min={key === 'width' || key === 'height' ? 1 : 0} max={100} precision={1} addonAfter="%" value={selected[key]} onChange={(value) => patchHotspot(selected.id, { [key]: Number(value ?? 0) })} /></Form.Item></Col>)}
            </Row>
          </Form>
        </div>
      ) : hotspots.length > 0 ? <Typography.Text className="hotspot-select-hint" type="secondary">点击图上的蓝色区域编辑动作和位置。</Typography.Text> : null}
    </div>
  );
}

function rectFromPoints(startX: number, startY: number, endX: number, endY: number): DrawState {
  return {
    startX,
    startY,
    x: Math.min(startX, endX),
    y: Math.min(startY, endY),
    width: Math.abs(endX - startX),
    height: Math.abs(endY - startY),
  };
}

function normalizeHotspot(value: ImageHotspot): ImageHotspot {
  const x = clamp(value.x, 0, 99);
  const y = clamp(value.y, 0, 99);
  return {
    ...value,
    x: round(x),
    y: round(y),
    width: round(clamp(value.width, 1, 100 - x)),
    height: round(clamp(value.height, 1, 100 - y)),
  };
}

function hotspotStyle(value: Pick<ImageHotspot, 'x' | 'y' | 'width' | 'height'>) {
  return { left: `${value.x}%`, top: `${value.y}%`, width: `${value.width}%`, height: `${value.height}%` };
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function round(value: number) {
  return Math.round(value * 1000) / 1000;
}
