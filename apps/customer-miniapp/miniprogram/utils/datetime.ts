const WALL_CLOCK = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/;

/** Formats an API datetime as a Beijing wall clock, regardless of the phone timezone. */
export function formatBeijingDateTime(value?: string | null): string {
  if (!value) return "--";
  const match = value.trim().match(WALL_CLOCK);
  if (match) return `${match[1]}-${match[2]}-${match[3]} ${match[4]}:${match[5]}:${match[6] || "00"}`;
  const instant = new Date(value);
  if (Number.isNaN(instant.getTime())) return "--";
  return formatUTCFields(new Date(instant.getTime() + 8 * 60 * 60 * 1000));
}

export function formatBeijingDate(value?: string | null): string {
  const formatted = formatBeijingDateTime(value);
  return formatted === "--" ? formatted : formatted.slice(0, 10);
}

export function beijingDateKey(now: Date | number = Date.now()): string {
  const timestamp = typeof now === "number" ? now : now.getTime();
  return formatUTCFields(new Date(timestamp + 8 * 60 * 60 * 1000)).slice(0, 10);
}

export function beijingNowDateTime(now: Date | number = Date.now()): string {
  const timestamp = typeof now === "number" ? now : now.getTime();
  return formatUTCFields(new Date(timestamp + 8 * 60 * 60 * 1000));
}

/** Parses a Beijing wall-clock value into an absolute timestamp. */
export function beijingEpoch(value?: string | null): number {
  if (!value) return Number.NaN;
  const match = value.trim().match(WALL_CLOCK);
  if (!match) return Date.parse(value);
  return Date.UTC(
    Number(match[1]), Number(match[2]) - 1, Number(match[3]),
    Number(match[4]) - 8, Number(match[5]), Number(match[6] || 0),
  );
}

function formatUTCFields(date: Date): string {
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`;
}
