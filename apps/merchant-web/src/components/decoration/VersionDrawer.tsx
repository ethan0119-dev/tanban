import { EyeOutlined, HistoryOutlined, RollbackOutlined } from '@ant-design/icons';
import { Button, Drawer, Empty, List, Space, Tag, Typography } from 'antd';
import type { DecorationVersion } from '../../features/decoration/model';

interface VersionDrawerProps {
  open: boolean;
  versions: DecorationVersion[];
  currentVersionNo?: number;
  loading: boolean;
  actionLoading: boolean;
  onClose: () => void;
  onLoad: (version: DecorationVersion) => void;
  onRollback: (version: DecorationVersion) => void;
}

export function VersionDrawer({ open, versions, currentVersionNo, loading, actionLoading, onClose, onLoad, onRollback }: VersionDrawerProps) {
  return (
    <Drawer title={<Space><HistoryOutlined />发布历史</Space>} width={460} open={open} onClose={onClose}>
      <Typography.Paragraph type="secondary">每次发布都会生成不可变版本。载入编辑器只改变本地草稿；回滚会基于旧版本生成新的发布版本。</Typography.Paragraph>
      {!loading && !versions.length
        ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无发布版本" />
        : <List
          loading={loading}
          dataSource={versions}
          renderItem={(version) => {
            const current = currentVersionNo === version.versionNo;
            return (
              <List.Item className="version-item">
                <div className="version-main">
                  <Space><strong>版本 V{version.versionNo}</strong>{current && <Tag color="green">当前线上</Tag>}</Space>
                  <p>{version.note || '未填写发布说明'}</p>
                  <small>{formatTime(version.publishedAt)}{version.publishedBy ? ` · ${version.publishedBy}` : ''}</small>
                </div>
                <Space direction="vertical" size={6}>
                  <Button size="small" icon={<EyeOutlined />} onClick={() => onLoad(version)}>载入</Button>
                  <Button size="small" icon={<RollbackOutlined />} loading={actionLoading} disabled={current} onClick={() => onRollback(version)}>回滚</Button>
                </Space>
              </List.Item>
            );
          }}
        />}
    </Drawer>
  );
}

function formatTime(value?: string) {
  if (!value) return '发布时间未知';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false });
}
