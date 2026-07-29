import type { TrendPoint } from '../types';

function pointsFor(values: number[], width: number, height: number): string {
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const range = Math.max(max - min, 1);
  return values
    .map((value, index) => {
      const x = values.length === 1 ? width / 2 : (index / (values.length - 1)) * width;
      const y = height - ((value - min) / range) * (height - 16) - 8;
      return `${x},${y}`;
    })
    .join(' ');
}

export function TrendChart({ data }: { data: TrendPoint[] }) {
  const values = data.map((item) => Number(item.orders || 0));
  const points = pointsFor(values, 720, 220);
  const fillPoints = `0,220 ${points} 720,220`;
  return (
    <div className="trend-chart">
      <div className="trend-chart__legend"><span />订单量</div>
      <svg viewBox="0 0 720 260" role="img" aria-label="近七日订单趋势">
        <defs>
          <linearGradient id="trendFill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="#f56a1d" stopOpacity="0.25" />
            <stop offset="100%" stopColor="#f56a1d" stopOpacity="0" />
          </linearGradient>
        </defs>
        {[40, 85, 130, 175, 220].map((y) => <line key={y} x1="0" y1={y} x2="720" y2={y} stroke="#ebe9e5" strokeDasharray="4 6" />)}
        <polygon points={fillPoints} fill="url(#trendFill)" />
        <polyline points={points} fill="none" stroke="#f56a1d" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" />
        {data.map((item, index) => {
          const x = data.length === 1 ? 360 : (index / (data.length - 1)) * 720;
          const max = Math.max(...values, 1);
          const min = Math.min(...values, 0);
          const y = 220 - ((values[index] - min) / Math.max(max - min, 1)) * 204;
          return (
            <g key={`${item.date}-${index}`}>
              <circle cx={x} cy={y} r="6" fill="#fff" stroke="#f56a1d" strokeWidth="3" />
              <text x={x} y="252" textAnchor="middle" fontSize="13" fill="#8b8580">{item.date.slice(5)}</text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
