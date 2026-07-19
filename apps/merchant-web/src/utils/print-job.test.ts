import { describe, expect, it } from 'vitest';
import { canRetryPrintJob, printJobStatusView } from './print-job';

describe('print job presentation', () => {
  it.each(['PROCESSING', 'PRINTING'] as const)('shows %s as processing', (status) => {
    expect(printJobStatusView(status)).toEqual({ text: '打印中', color: 'processing' });
    expect(canRetryPrintJob(status)).toBe(false);
  });

  it('surfaces an unknown result and allows an explicit reprint', () => {
    expect(printJobStatusView('UNKNOWN')).toEqual({ text: '结果待确认', color: 'warning' });
    expect(canRetryPrintJob('UNKNOWN')).toBe(true);
  });
});
