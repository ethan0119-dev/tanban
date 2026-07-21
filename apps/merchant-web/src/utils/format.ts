import dayjs, { type Dayjs } from 'dayjs';
import customParseFormat from 'dayjs/plugin/customParseFormat';

dayjs.extend(customParseFormat);

export function yuan(value?: number | string | null): string {
  const numberValue = Number(value ?? 0);
  return Number.isFinite(numberValue) ? `¥${numberValue.toFixed(2)}` : '¥0.00';
}

export function dateTime(value?: string | null): string {
  if (!value) return '--';
  const wallClock = beijingWallClock(value);
  if (wallClock) return wallClock;
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return '--';
  return formatShiftedUTC(instant.getTime() + 8 * 60 * 60 * 1000);
}

export function dateOnly(value?: string | null): string {
  const formatted = dateTime(value);
  return formatted === '--' ? formatted : formatted.slice(0, 10);
}

/** Converts an API datetime into the wall-clock value used by Beijing-time pickers. */
export function beijingPickerValue(value?: string | null): Dayjs | null {
  const formatted = dateTime(value);
  if (formatted === '--') return null;
  const parsed = dayjs(formatted, 'YYYY-MM-DD HH:mm:ss', true);
  return parsed.isValid() ? parsed : null;
}

/** Serializes picker fields explicitly as UTC+8, independent of the browser timezone. */
export function toBeijingRFC3339(value?: Dayjs | null): string | undefined {
  return value?.isValid() ? `${value.format('YYYY-MM-DDTHH:mm:ss')}+08:00` : undefined;
}

export function beijingNowDateTime(now = new Date()): string {
  return formatShiftedUTC(now.getTime() + 8 * 60 * 60 * 1000);
}

export function beijingTodayKey(now = new Date()): string {
  return beijingNowDateTime(now).slice(0, 10);
}

function beijingWallClock(value: string): string | null {
  const match = value.trim().match(/^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/);
  if (!match) return null;
  return `${match[1]}-${match[2]}-${match[3]} ${match[4]}:${match[5]}:${match[6] || '00'}`;
}

function formatShiftedUTC(timestamp: number): string {
  const date = new Date(timestamp);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`;
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
