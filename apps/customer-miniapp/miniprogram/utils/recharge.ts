export function yuanInputToCents(value: string): number {
  const normalized = value.trim();
  if (!/^\d{1,7}(?:\.\d{0,2})?$/.test(normalized)) return 0;
  const [yuan, fraction = ""] = normalized.split(".");
  return Number(yuan) * 100 + Number((fraction + "00").slice(0, 2));
}

export function yuanText(cents: number): string {
  return (cents / 100).toFixed(cents % 100 ? 2 : 0);
}

export function customRechargeAllowed(amountCents: number, minRechargeCents: number, maxRechargeCents: number): boolean {
  return amountCents >= minRechargeCents && amountCents <= maxRechargeCents;
}

