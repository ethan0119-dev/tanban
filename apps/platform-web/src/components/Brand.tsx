/* eslint-disable @next/next/no-img-element -- this Vite app imports a fingerprinted local brand asset */
import tanbanLogo from '../assets/brand/tanban-logo-web.png';

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`brand ${compact ? 'brand--compact' : ''}`} aria-label="摊伴">
      <img src={tanbanLogo} alt="摊伴 TANBAN" />
    </div>
  );
}
