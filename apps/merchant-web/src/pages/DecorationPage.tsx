import {
  CloudUploadOutlined,
  EyeOutlined,
  HistoryOutlined,
  ReloadOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Input, Modal, Skeleton, Space, Tabs, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { AssetLibraryPanel } from '../components/decoration/AssetLibraryPanel';
import {
  NavigationPanel,
  OrderingPanel,
  PageModulesPanel,
  SplashPanel,
  TemplatePanel,
  ThemePanel,
} from '../components/decoration/DecorationEditorPanels';
import { PhonePreview } from '../components/decoration/PhonePreview';
import { VersionDrawer } from '../components/decoration/VersionDrawer';
import { PageHeading } from '../components/PageHeading';
import { decorationApi } from '../features/decoration/api';
import { BUILTIN_TEMPLATES, cloneDecoration, DEFAULT_DECORATION } from '../features/decoration/defaults';
import type {
  DecorationConfig,
  DecorationDraft,
  DecorationTemplate,
  DecorationVersion,
  DecorationWorkspace,
  MediaAsset,
  PreviewPage,
} from '../features/decoration/model';
import '../decoration.css';

type DecorationTab = 'pages' | 'theme' | 'ordering' | 'templates' | 'navigation' | 'splash' | 'assets';

const tabLabels: Array<{ key: DecorationTab; label: string }> = [
  { key: 'pages', label: '页面编排' },
  { key: 'theme', label: '全店风格' },
  { key: 'ordering', label: '点单风格' },
  { key: 'templates', label: '装修模板' },
  { key: 'navigation', label: '导航设置' },
  { key: 'splash', label: '启动页设置' },
  { key: 'assets', label: '素材管理' },
];

export function DecorationPage() {
  const { user } = useAuth();
  const [config, setConfig] = useState<DecorationConfig>(() => cloneDecoration(DEFAULT_DECORATION));
  const [workspace, setWorkspace] = useState<DecorationWorkspace | null>(null);
  const [templates, setTemplates] = useState<DecorationTemplate[]>(BUILTIN_TEMPLATES);
  const [versions, setVersions] = useState<DecorationVersion[]>([]);
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [activeTab, setActiveTab] = useState<DecorationTab>('pages');
  const [previewPage, setPreviewPage] = useState<PreviewPage>('HOME');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [templatesLoading, setTemplatesLoading] = useState(false);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [versionActionLoading, setVersionActionLoading] = useState(false);
  const [assetsLoading, setAssetsLoading] = useState(false);
  const [assetSaving, setAssetSaving] = useState(false);
  const [assetModalOpen, setAssetModalOpen] = useState(false);
  const [editingAsset, setEditingAsset] = useState<MediaAsset | null>(null);
  const [versionsOpen, setVersionsOpen] = useState(false);
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishNote, setPublishNote] = useState('');
  const [dirty, setDirty] = useState(false);
  const [loadWarning, setLoadWarning] = useState('');
  const [messageApi, contextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();

  const revision = workspace?.draft.revision ?? 0;
  const publishedVersionNo = workspace?.published?.versionNo;

  const loadAssets = useCallback(async () => {
    setAssetsLoading(true);
    try {
      setAssets(await decorationApi.listAssets());
    } catch (error) {
      messageApi.warning(`素材库加载失败：${errorMessage(error)}`);
    } finally {
      setAssetsLoading(false);
    }
  }, [messageApi]);

  const loadTemplates = useCallback(async () => {
    setTemplatesLoading(true);
    try {
      const remote = await decorationApi.listTemplates();
      const merged = new Map(BUILTIN_TEMPLATES.map((item) => [item.key, item]));
      remote.forEach((item) => merged.set(item.key, item));
      setTemplates([...merged.values()]);
    } catch (error) {
      setTemplates(BUILTIN_TEMPLATES);
      messageApi.warning(`在线模板暂不可用，已加载内置模板：${errorMessage(error)}`);
    } finally {
      setTemplatesLoading(false);
    }
  }, [messageApi]);

  const loadWorkspace = useCallback(async () => {
    setLoading(true);
    setLoadWarning('');
    try {
      const next = await decorationApi.loadWorkspace();
      setWorkspace(next);
      setConfig(cloneDecoration(next.draft.config));
      setDirty(false);
    } catch (error) {
      setWorkspace({ draft: { revision: 0, config: cloneDecoration(DEFAULT_DECORATION) }, published: null });
      setConfig(cloneDecoration(DEFAULT_DECORATION));
      setDirty(false);
      setLoadWarning(`服务器草稿暂未加载，当前展示内置默认方案。保存前请先确认接口状态：${errorMessage(error)}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void Promise.all([loadWorkspace(), loadTemplates(), loadAssets()]);
  }, [loadAssets, loadTemplates, loadWorkspace]);

  const updateConfig = useCallback((next: DecorationConfig) => {
    setConfig(next);
    setDirty(true);
  }, []);

  const persistDraft = useCallback(async (): Promise<DecorationDraft> => {
    const response = await decorationApi.saveDraft(revision, config);
    response.config.storeName = config.storeName;
    const draft = { ...response, updatedAt: response.updatedAt || new Date().toISOString() };
    setWorkspace((current) => ({ draft, published: current?.published ?? null }));
    setConfig(cloneDecoration(draft.config));
    setDirty(false);
    setLoadWarning('');
    return draft;
  }, [config, revision]);

  const save = async () => {
    setSaving(true);
    try {
      await persistDraft();
      messageApi.success('装修草稿已保存，线上页面未发生变化');
    } catch (error) {
      messageApi.error(withRevisionHint(error));
    } finally {
      setSaving(false);
    }
  };

  const publish = async () => {
    setPublishing(true);
    try {
      let expectedRevision = revision;
      if (dirty || expectedRevision === 0) {
        const saved = await persistDraft();
        expectedRevision = saved.revision;
      }
      const published = await decorationApi.publish(expectedRevision, publishNote.trim());
      setWorkspace((current) => current ? { ...current, published } : { draft: { revision: expectedRevision, config }, published });
      setDirty(false);
      setPublishOpen(false);
      setPublishNote('');
      messageApi.success(`已发布 V${published.versionNo}，顾客端将读取最新配置`);
      if (versionsOpen) void loadVersions();
    } catch (error) {
      messageApi.error(withRevisionHint(error));
    } finally {
      setPublishing(false);
    }
  };

  const loadVersions = useCallback(async () => {
    setVersionsLoading(true);
    try {
      setVersions(await decorationApi.listVersions());
    } catch (error) {
      messageApi.error(`发布历史加载失败：${errorMessage(error)}`);
    } finally {
      setVersionsLoading(false);
    }
  }, [messageApi]);

  const openVersions = () => {
    setVersionsOpen(true);
    void loadVersions();
  };

  const loadVersionIntoEditor = async (version: DecorationVersion) => {
    setVersionActionLoading(true);
    try {
      const detail = version.config ? version : await decorationApi.getVersion(version.id);
      if (!detail.config) throw new Error('该历史版本没有可用配置');
      setConfig(cloneDecoration(detail.config));
      setDirty(true);
      setVersionsOpen(false);
      messageApi.success(`V${version.versionNo} 已载入编辑器，保存后才会成为新草稿`);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setVersionActionLoading(false);
    }
  };

  const confirmRollback = (version: DecorationVersion) => {
    modal.confirm({
      title: `回滚到 V${version.versionNo}？`,
      content: '系统不会覆盖历史版本，而是基于该配置生成一个新的发布版本。当前未保存的编辑内容将被替换。',
      okText: '确认回滚并发布',
      cancelText: '取消',
      onOk: async () => {
        setVersionActionLoading(true);
        try {
          const published = await decorationApi.rollback(version.id, revision, `回滚至 V${version.versionNo}`);
          const refreshed = await decorationApi.loadWorkspace();
          setWorkspace(refreshed);
          setConfig(cloneDecoration(refreshed.draft.config));
          setDirty(false);
          await loadVersions();
          messageApi.success(`已回滚并发布 V${published.versionNo}`);
        } catch (error) {
          messageApi.error(withRevisionHint(error));
          throw error;
        } finally {
          setVersionActionLoading(false);
        }
      },
    });
  };

  const applyTemplate = (template: DecorationTemplate) => {
    const currentHero = config.homeModules.find((item) => item.type === 'HERO_BANNER');
    const next = cloneDecoration(template.config);
    next.storeName = config.storeName;
    if (currentHero?.imageUrl) {
      next.homeModules = next.homeModules.map((item) => item.type === 'HERO_BANNER' ? { ...item, imageUrl: currentHero.imageUrl, title: currentHero.title, subtitle: currentHero.subtitle } : item);
    }
    updateConfig({ ...next, templateKey: template.key });
    messageApi.success(`已套用“${template.name}”，保存草稿后生效`);
  };

  const createAsset = async (input: Pick<MediaAsset, 'name' | 'url' | 'type'>) => {
    setAssetSaving(true);
    try {
      const created = await decorationApi.createAsset(input);
      setAssets((items) => [created, ...items]);
      setAssetModalOpen(false);
      messageApi.success('素材已录入');
    } catch (error) {
      messageApi.error(errorMessage(error));
      throw error;
    } finally {
      setAssetSaving(false);
    }
  };

  const updateAsset = async (id: string | number, input: Pick<MediaAsset, 'name' | 'url' | 'type'>) => {
    setAssetSaving(true);
    try {
      const updated = await decorationApi.updateAsset(id, input);
      setAssets((items) => items.map((item) => item.id === id ? updated : item));
      setAssetModalOpen(false);
      setEditingAsset(null);
      messageApi.success('素材已更新');
    } catch (error) {
      messageApi.error(errorMessage(error));
      throw error;
    } finally {
      setAssetSaving(false);
    }
  };

  const deleteAsset = async (asset: MediaAsset) => {
    try {
      await decorationApi.deleteAsset(asset.id);
      setAssets((items) => items.filter((item) => item.id !== asset.id));
      messageApi.success('素材已删除');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const useAsset = (asset: MediaAsset, target: 'BANNER' | 'SPLASH') => {
    if (asset.type !== 'IMAGE') {
      messageApi.warning('当前目标只支持图片素材');
      return;
    }
    if (target === 'SPLASH') {
      updateConfig({ ...config, splash: { ...config.splash, imageUrl: asset.url } });
      setActiveTab('splash');
    } else {
      updateConfig({ ...config, homeModules: config.homeModules.map((item) => item.type === 'HERO_BANNER' ? { ...item, imageUrl: asset.url } : item) });
      setActiveTab('pages');
    }
    messageApi.success(`已将“${asset.name}”应用到${target === 'BANNER' ? '首页 Banner' : '启动页'}`);
  };

  const items = useMemo(() => tabLabels.map((tab) => ({
    key: tab.key,
    label: tab.label,
    children: tab.key === 'pages' ? <PageModulesPanel config={config} onChange={updateConfig} />
      : tab.key === 'theme' ? <ThemePanel config={config} onChange={updateConfig} />
        : tab.key === 'ordering' ? <OrderingPanel config={config} onChange={updateConfig} />
          : tab.key === 'templates' ? <TemplatePanel config={config} templates={templates} loading={templatesLoading} onApply={applyTemplate} />
            : tab.key === 'navigation' ? <NavigationPanel config={config} onChange={updateConfig} />
              : tab.key === 'splash' ? <SplashPanel config={config} onChange={updateConfig} />
                : <AssetLibraryPanel
                  assets={assets}
                  loading={assetsLoading}
                  saving={assetSaving}
                  editing={editingAsset}
                  modalOpen={assetModalOpen}
                  onCreate={createAsset}
                  onUpdate={updateAsset}
                  onDelete={deleteAsset}
                  onUse={useAsset}
                  onOpen={(asset) => { setEditingAsset(asset ?? null); setAssetModalOpen(true); }}
                  onClose={() => { setAssetModalOpen(false); setEditingAsset(null); }}
                />,
  })), [activeTab, assetModalOpen, assetSaving, assets, assetsLoading, config, editingAsset, templates, templatesLoading, updateConfig]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <div className="page-shell decoration-page">
      {contextHolder}
      {modalContextHolder}
      <PageHeading
        title="小程序装修"
        description="配置门店首页、点单风格与品牌素材；保存为草稿，确认预览后再发布"
        extra={(
          <Space wrap>
            {dirty ? <Tag color="orange">有未保存修改</Tag> : <Tag color="green">草稿已保存</Tag>}
            {publishedVersionNo ? <Tag>线上 V{publishedVersionNo}</Tag> : <Tag>尚未发布</Tag>}
            <Button icon={<HistoryOutlined />} onClick={openVersions}>发布历史</Button>
            <Button icon={<SaveOutlined />} loading={saving} disabled={loading || !dirty} onClick={() => void save()}>保存草稿</Button>
            <Button type="primary" icon={<CloudUploadOutlined />} loading={publishing} disabled={loading} onClick={() => setPublishOpen(true)}>发布上线</Button>
          </Space>
        )}
      />

      {loadWarning && <Alert className="decoration-load-warning" type="warning" showIcon message="当前使用默认装修配置" description={loadWarning} action={<Button size="small" icon={<ReloadOutlined />} onClick={() => void loadWorkspace()}>重新加载</Button>} />}
      <div className="decoration-status-strip">
        <span><b>草稿版本</b> R{revision}</span>
        <span><b>最后保存</b> {formatTime(workspace?.draft.updatedAt)}</span>
        <span><b>线上版本</b> {workspace?.published ? `V${workspace.published.versionNo} · ${formatTime(workspace.published.publishedAt)}` : '暂无'}</span>
        <span className="status-help"><EyeOutlined /> 右侧为即时模拟效果</span>
      </div>

      {loading
        ? <Card className="content-card"><Skeleton active paragraph={{ rows: 12 }} /></Card>
        : <div className="decoration-workbench">
          <Card className="content-card decoration-editor-card" styles={{ body: { padding: 0 } }}>
            <Tabs
              tabPosition="left"
              activeKey={activeTab}
              items={items}
              onChange={(key) => {
                const next = key as DecorationTab;
                setActiveTab(next);
                if (next === 'ordering') setPreviewPage('MENU');
                if (next === 'pages' || next === 'theme' || next === 'splash') setPreviewPage('HOME');
              }}
            />
          </Card>
          <PhonePreview config={config} storeName={config.storeName || user?.storeName || user?.merchantName || '我的门店'} page={previewPage} onPageChange={setPreviewPage} showSplash={activeTab === 'splash'} />
        </div>}

      <VersionDrawer
        open={versionsOpen}
        versions={versions}
        currentVersionNo={publishedVersionNo}
        loading={versionsLoading}
        actionLoading={versionActionLoading}
        onClose={() => setVersionsOpen(false)}
        onLoad={(version) => void loadVersionIntoEditor(version)}
        onRollback={confirmRollback}
      />

      <Modal
        title="发布小程序装修"
        open={publishOpen}
        okText="确认发布"
        cancelText="继续编辑"
        confirmLoading={publishing}
        onOk={() => void publish()}
        onCancel={() => setPublishOpen(false)}
      >
        <Alert type="info" showIcon message={dirty ? '系统会先保存当前草稿，再生成不可变的线上版本。' : '系统将基于当前草稿生成一个不可变的线上版本。'} />
        <Typography.Paragraph className="publish-note-label">发布说明</Typography.Paragraph>
        <Input.TextArea rows={3} maxLength={100} showCount value={publishNote} placeholder="例如：夏季菜单首页改版" onChange={(event) => setPublishNote(event.target.value)} />
      </Modal>
    </div>
  );
}

function formatTime(value?: string) {
  if (!value) return '尚未保存';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false });
}

function withRevisionHint(error: unknown) {
  const message = errorMessage(error);
  return /revision|冲突|conflict|409/i.test(message) ? `${message}。草稿可能已被其他员工更新，请重新加载后再操作。` : message;
}
