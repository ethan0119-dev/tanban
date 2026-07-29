/* eslint-disable @next/next/no-img-element -- this Vite app imports a fingerprinted local brand asset */
import tanbanIcon from '../assets/brand/tanban-icon.png';

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`brand ${compact ? 'brand--compact' : ''}`} aria-label="摊伴">
      <span className="brand__mark"><img src={tanbanIcon} alt="" /></span>
      {!compact && <span className="brand__word"><strong>摊伴</strong><small>TANBAN</small></span>}
    </div>
  );
}
