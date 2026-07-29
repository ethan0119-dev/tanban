import { pinyin } from 'pinyin-pro';

export type OwnerUsernameMode = 'PHONE' | 'PINYIN' | 'CUSTOM';

export function usernameFromStoreName(storeName: string, fallback = 'shop'): string {
  const converted = pinyin(storeName.trim(), { toneType: 'none', type: 'array' })
    .join('')
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '');
  const normalizedFallback = fallback.toLowerCase().replace(/[^a-z0-9]/g, '') || 'shop';
  return (converted || normalizedFallback).slice(0, 48);
}

export function generatedOwnerUsername(mode: OwnerUsernameMode, storeName: string, phone: string, fallback = 'shop'): string {
  if (mode === 'PHONE') return phone.replace(/\D/g, '').slice(0, 32);
  if (mode === 'PINYIN') return usernameFromStoreName(storeName, fallback);
  return '';
}

export function generateInitialPassword(length = 14): string {
  const groups = ['abcdefghijkmnopqrstuvwxyz', 'ABCDEFGHJKLMNPQRSTUVWXYZ', '23456789', '!@#$%'];
  const randomIndex = (limit: number) => {
    // Rejection sampling avoids modulo bias while keeping all generated
    // credentials in the browser; the API only ever receives the plaintext
    // once and persists a bcrypt hash.
    const ceiling = Math.floor(0x1_0000_0000 / limit) * limit;
    const buffer = new Uint32Array(1);
    do crypto.getRandomValues(buffer); while (buffer[0] >= ceiling);
    return buffer[0] % limit;
  };
  const chars = groups.map((group) => group[randomIndex(group.length)]);
  const alphabet = groups.join('');
  while (chars.length < Math.max(length, 8)) chars.push(alphabet[randomIndex(alphabet.length)]);
  for (let index = chars.length - 1; index > 0; index -= 1) {
    const target = randomIndex(index + 1);
    [chars[index], chars[target]] = [chars[target], chars[index]];
  }
  return chars.join('');
}
