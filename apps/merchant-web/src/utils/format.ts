import dayjs, { type Dayjs } from 'dayjs';
import customParseFormat from 'dayjs/plugin/customParseFormat';

dayjs.extend(customParseFormat);

export function yuan(value?: number | string | null): string {
  const numberValue = Number(value ?? 0);
  return Number.isFinite(numberValue) ? `¥${numberValue.toFixed(2)}` : '¥0.00';
}

export function dateTime(value?: string | null): string {
  if (!value) return '--';
  const date = dayjs(value);
  return date.isValid() ? date.format('YYYY-MM-DD HH:mm:ss') : '--';
}

export function percentChange(current?: number, previous?: number): number | null {
  if (!previous) return null;
  return Number((((current ?? 0) - previous) / previous * 100).toFixed(1));
}

export function initials(name?: string): string {
  return name?.trim().slice(0, 1).toUpperCase() || '摊';
}

export function businessHoursRange(values?: readonly string[]): [Dayjs, Dayjs] | undefined {
  if (!values || values.length !== 2) return undefined;
  const start = dayjs(values[0], 'HH:mm', true);
  const end = dayjs(values[1], 'HH:mm', true);
  return start.isValid() && end.isValid() ? [start, end] : undefined;
}

export function pickupCode(orderId: string | number | null | undefined): string {
  const numericId = Number(orderId);
  if (!Number.isFinite(numericId)) return '';
  const normalized = ((Math.trunc(numericId) % 10_000) + 10_000) % 10_000;
  return String(normalized).padStart(4, '0');
}
