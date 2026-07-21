const WALL_CLOCK = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/;

export function formatBeijingDateTime(value?: string | null): string {
  if (!value) return '—';
  const match = value.trim().match(WALL_CLOCK);
  if (match) return `${match[1]}-${match[2]}-${match[3]} ${match[4]}:${match[5]}:${match[6] || '00'}`;
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return '—';
  const shifted = new Date(instant.getTime() + 8 * 60 * 60 * 1000);
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${shifted.getUTCFullYear()}-${pad(shifted.getUTCMonth() + 1)}-${pad(shifted.getUTCDate())} ${pad(shifted.getUTCHours())}:${pad(shifted.getUTCMinutes())}:${pad(shifted.getUTCSeconds())}`;
}

export function formatBeijingDate(value?: string | null): string {
  const formatted = formatBeijingDateTime(value);
  return formatted === '—' ? formatted : formatted.slice(0, 10);
}
