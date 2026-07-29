import { Alert, Button, Empty, Skeleton } from 'antd';

export function PageSkeleton() {
  return (
    <div className="page-skeleton">
      <Skeleton active paragraph={{ rows: 2 }} />
      <div className="page-skeleton__grid">
        {[1, 2, 3, 4].map((item) => <Skeleton.Node active key={item} />)}
      </div>
      <Skeleton active paragraph={{ rows: 8 }} />
    </div>
  );
}

export function LoadError({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <Alert
      showIcon
      type="error"
      message="页面加载失败"
      description={message || '暂时无法获取数据，请稍后重试。'}
      action={onRetry ? <Button size="small" onClick={onRetry}>重新加载</Button> : undefined}
    />
  );
}

export function EmptyData({ description = '暂无数据' }: { description?: string }) {
  return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description} />;
}
