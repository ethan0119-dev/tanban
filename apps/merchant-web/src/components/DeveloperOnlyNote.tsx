import { Alert, type AlertProps } from 'antd';
import type { ReactNode } from 'react';

export function DeveloperOnlyNote({ children, className, style }: { children: ReactNode; className?: string; style?: AlertProps['style'] }) {
  if (!import.meta.env.DEV) return null;
  return <Alert className={className} style={style} type="warning" showIcon message="开发提示（正式构建不会显示）" description={children} />;
}
