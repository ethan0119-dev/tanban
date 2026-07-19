import { CoffeeOutlined } from '@ant-design/icons';

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`brand ${compact ? 'brand--compact' : ''}`} aria-label="摊伴">
      <span className="brand__mark"><CoffeeOutlined /></span>
      {!compact && (
        <span className="brand__text">
          <strong>摊伴</strong>
          <small>TANBAN SaaS</small>
        </span>
      )}
    </div>
  );
}
