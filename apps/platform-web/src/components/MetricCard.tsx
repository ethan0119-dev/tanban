import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons';
import { Card, Statistic } from 'antd';
import type { ReactNode } from 'react';

export function MetricCard({
  title,
  value,
  prefix,
  suffix,
  trend,
  accent = 'orange',
}: {
  title: string;
  value: number;
  prefix?: ReactNode;
  suffix?: string;
  trend?: number;
  accent?: 'orange' | 'green' | 'blue' | 'purple';
}) {
  return (
    <Card className={`metric-card metric-card--${accent}`} bordered={false}>
      <Statistic title={title} value={value} prefix={prefix} suffix={suffix} precision={suffix === '元' ? 2 : 0} />
      {trend !== undefined && (
        <div className={`metric-card__trend ${trend >= 0 ? 'is-up' : 'is-down'}`}>
          {trend >= 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />}
          <span>{Math.abs(trend)}%</span>
          <small>较昨日</small>
        </div>
      )}
    </Card>
  );
}
