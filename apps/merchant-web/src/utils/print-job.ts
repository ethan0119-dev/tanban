import type { PrintJob } from '../types';

export interface PrintJobStatusView {
  text: string;
  color: 'success' | 'error' | 'processing' | 'warning' | 'default';
}

export function printJobStatusView(status: PrintJob['status']): PrintJobStatusView {
  switch (status) {
    case 'SUCCESS': return { text: '打印成功', color: 'success' };
    case 'FAILED': return { text: '打印失败', color: 'error' };
    case 'PROCESSING':
    case 'PRINTING': return { text: '打印中', color: 'processing' };
    case 'UNKNOWN': return { text: '结果待确认', color: 'warning' };
    default: return { text: '等待中', color: 'default' };
  }
}

export function canRetryPrintJob(status: PrintJob['status']): boolean {
  return status === 'FAILED' || status === 'SUCCESS' || status === 'UNKNOWN';
}
