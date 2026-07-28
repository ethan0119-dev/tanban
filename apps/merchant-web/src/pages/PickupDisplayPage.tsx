import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  HourglassOutlined,
  MutedOutlined,
  ReloadOutlined,
  SoundOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import { Button } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import './pickup-display.css';

export type PickupDisplayLayout = 'landscape' | 'portrait';

export interface PickupDisplayOrder {
  id: string | number;
  pickupCode: string;
  status: 'PAID' | 'ACCEPTED' | 'PREPARING' | 'READY';
  createdAt?: string;
  updatedAt?: string;
}

export interface PickupDisplayData {
  storeName: string;
  storeLogoUrl?: string;
  businessDate: string;
  updatedAt: string;
  preparing: PickupDisplayOrder[];
  ready: PickupDisplayOrder[];
}

const previewDisplay: PickupDisplayData = {
  storeName: '码农咖啡鼓楼店',
  businessDate: '2026-07-26',
  updatedAt: '2026-07-26 16:33:28',
  preparing: [
    { id: 1, pickupCode: 'A012', status: 'PREPARING' },
    { id: 2, pickupCode: 'A013', status: 'PAID' },
    { id: 3, pickupCode: 'A015', status: 'ACCEPTED' },
    { id: 4, pickupCode: 'A018', status: 'PREPARING' },
    { id: 5, pickupCode: 'A020', status: 'PREPARING' },
  ],
  ready: [
    { id: 6, pickupCode: 'A008', status: 'READY' },
    { id: 7, pickupCode: 'A010', status: 'READY' },
    { id: 8, pickupCode: 'A011', status: 'READY' },
  ],
};

const VOICE_STORAGE_KEY = 'pickup-display-voice';

export function normalizePickupDisplayLayout(value: string | null | undefined): PickupDisplayLayout {
  return value === 'portrait' ? 'portrait' : 'landscape';
}

function formatClock(value: Date): string {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(value);
}

function formatDate(value: string): string {
  const date = new Date(`${value}T00:00:00+08:00`);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: 'long',
    day: 'numeric',
    weekday: 'short',
  }).format(date);
}

function pageOf<T>(items: T[], pageSize: number, rotation: number): { items: T[]; page: number; pages: number } {
  const pages = Math.max(1, Math.ceil(items.length / pageSize));
  const page = rotation % pages;
  return { items: items.slice(page * pageSize, (page + 1) * pageSize), page, pages };
}

function PickupColumn({
  kind,
  orders,
  page,
  pages,
  recentReady,
}: {
  kind: 'preparing' | 'ready';
  orders: PickupDisplayOrder[];
  page: number;
  pages: number;
  recentReady: Set<string>;
}) {
  const ready = kind === 'ready';
  return (
    <section className={`pickup-display-column is-${kind}`} aria-label={ready ? '请取餐号码' : '制作中号码'}>
      <header className="pickup-display-column-heading">
        <span className="pickup-display-column-icon">{ready ? <CheckCircleOutlined /> : <HourglassOutlined />}</span>
        <div>
          <h2>{ready ? '请取餐' : '制作中'}</h2>
          <p>{ready ? '餐品已完成，请到取餐处领取' : '请耐心等待，我们正在为您制作'}</p>
        </div>
        {pages > 1 && <span className="pickup-display-page-index">{page + 1}/{pages}</span>}
      </header>
      {orders.length ? (
        <div className="pickup-display-number-grid">
          {orders.map((order) => (
            <div
              className={`pickup-display-number${recentReady.has(order.pickupCode) ? ' is-new' : ''}`}
              key={String(order.id)}
              aria-label={`取餐号 ${order.pickupCode}`}
            >
              {order.pickupCode}
            </div>
          ))}
        </div>
      ) : (
        <div className="pickup-display-empty">
          <strong>{ready ? '暂时没有待取餐订单' : '当前没有制作中订单'}</strong>
          <span>{ready ? '餐品完成后，取餐号会显示在这里' : '新订单进入制作后会自动显示'}</span>
        </div>
      )}
    </section>
  );
}

export function PickupDisplayPage({ previewMode = false }: { previewMode?: boolean }) {
  const { user } = useAuth();
  const [searchParams] = useSearchParams();
  const layout = normalizePickupDisplayLayout(searchParams.get('layout'));
  const [data, setData] = useState<PickupDisplayData>(previewMode ? previewDisplay : {
    storeName: user?.storeName || '取餐大屏',
    businessDate: '',
    updatedAt: '',
    preparing: [],
    ready: [],
  });
  const [loading, setLoading] = useState(!previewMode);
  const [connectionError, setConnectionError] = useState('');
  const [now, setNow] = useState(() => previewMode ? new Date('2026-07-26T16:33:00+08:00') : new Date());
  const [rotation, setRotation] = useState(0);
  const [fullscreen, setFullscreen] = useState(Boolean(document.fullscreenElement));
  const [recentReady, setRecentReady] = useState<Set<string>>(new Set());
  const previousReady = useRef<Set<string> | null>(null);
  const highlightTimer = useRef<number | null>(null);
  const [voiceEnabled, setVoiceEnabled] = useState(() => localStorage.getItem(VOICE_STORAGE_KEY) === '1');
  const voiceRef = useRef<SpeechSynthesisVoice | null>(null);
  const speechQueue = useRef<string[]>([]);
  const speaking = useRef(false);

  const pickVoice = useCallback(() => {
    const voices = window.speechSynthesis?.getVoices() ?? [];
    const zhVoices = voices.filter((v) => v.lang.startsWith('zh'));
    voiceRef.current = zhVoices.find((v) => v.name.includes('女') || v.name.includes('Female') || v.name.includes('Xiaoxiao') || v.name.includes('Yaoyao')) ?? zhVoices[0] ?? null;
  }, []);

  const speakNext = useCallback(() => {
    if (!voiceEnabled || speaking.current || !speechQueue.current.length) return;
    const code = speechQueue.current.shift();
    if (!code) return;
    speaking.current = true;
    const utter = new SpeechSynthesisUtterance(`请${code}号顾客取餐`);
    utter.lang = 'zh-CN';
    utter.rate = 0.95;
    utter.pitch = 1.1;
    if (voiceRef.current) utter.voice = voiceRef.current;
    utter.onend = () => { speaking.current = false; speakNext(); };
    utter.onerror = () => { speaking.current = false; speakNext(); };
    window.speechSynthesis.speak(utter);
  }, [voiceEnabled]);

  const speak = useCallback((codes: Set<string>) => {
    if (!voiceEnabled || !window.speechSynthesis) return;
    for (const code of codes) speechQueue.current.push(code);
    speakNext();
  }, [voiceEnabled, speakNext]);

  const toggleVoice = useCallback(() => {
    setVoiceEnabled((prev) => {
      const next = !prev;
      localStorage.setItem(VOICE_STORAGE_KEY, next ? '1' : '0');
      if (!next) { window.speechSynthesis?.cancel(); speechQueue.current = []; }
      return next;
    });
  }, []);

  useEffect(() => {
    pickVoice();
    window.speechSynthesis?.addEventListener('voiceschanged', pickVoice);
    return () => window.speechSynthesis?.removeEventListener('voiceschanged', pickVoice);
  }, [pickVoice]);

  const load = useCallback(async () => {
    if (previewMode) return;
    try {
      const next = await api.get<PickupDisplayData>('/merchant/pickup-display');
      const currentReady = new Set((next.ready || []).map((item) => item.pickupCode));
      if (previousReady.current) {
        const added = new Set([...currentReady].filter((code) => !previousReady.current?.has(code)));
        if (added.size) {
          setRecentReady(added);
          if (highlightTimer.current) window.clearTimeout(highlightTimer.current);
          highlightTimer.current = window.setTimeout(() => setRecentReady(new Set()), 12_000);
          speak(added);
        }
      }
      previousReady.current = currentReady;
      setData({ ...next, preparing: next.preparing || [], ready: next.ready || [] });
      setConnectionError('');
    } catch (error) {
      setConnectionError(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [previewMode]);

  useEffect(() => {
    void load();
    if (previewMode) return;
    const timer = window.setInterval(() => void load(), 5_000);
    const onVisible = () => { if (document.visibilityState === 'visible') void load(); };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [load, previewMode]);

  useEffect(() => {
    const clock = window.setInterval(() => setNow(new Date()), 1_000);
    const pages = window.setInterval(() => setRotation((value) => value + 1), 8_000);
    return () => {
      window.clearInterval(clock);
      window.clearInterval(pages);
      if (highlightTimer.current) window.clearTimeout(highlightTimer.current);
    };
  }, []);

  useEffect(() => {
    const onFullscreen = () => setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onFullscreen);
    return () => document.removeEventListener('fullscreenchange', onFullscreen);
  }, []);

  useEffect(() => {
    document.title = `${data.storeName || '门店'} · 取餐大屏`;
  }, [data.storeName]);

  const pageSize = layout === 'portrait' ? 8 : 12;
  const preparing = useMemo(() => pageOf(data.preparing, pageSize, rotation), [data.preparing, pageSize, rotation]);
  const ready = useMemo(() => pageOf(data.ready, pageSize, rotation), [data.ready, pageSize, rotation]);
  const activeCount = data.preparing.length + data.ready.length;

  const toggleFullscreen = async () => {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await document.documentElement.requestFullscreen();
    } catch {
      setConnectionError('浏览器未允许自动全屏，请使用浏览器菜单进入全屏');
    }
  };

  return (
    <main className="pickup-display-page" data-layout={layout}>
      <header className="pickup-display-header">
        <div className="pickup-display-store">
          {data.storeLogoUrl && <img src={data.storeLogoUrl} alt="" />}
          <div>
            <span>取餐进度 · 实时更新</span>
            <h1>{data.storeName || user?.storeName || '取餐大屏'}</h1>
          </div>
        </div>
        <div className="pickup-display-summary">
          <strong>{formatClock(now)}</strong>
          <span>{data.businessDate ? formatDate(data.businessDate) : '正在读取营业日期'} · {activeCount} 单处理中</span>
        </div>
        <Button
          className="pickup-display-fullscreen"
          type="text"
          icon={voiceEnabled ? <SoundOutlined /> : <MutedOutlined />}
          onClick={toggleVoice}
        >
          {voiceEnabled ? '语音播报中' : '开启语音播报'}
        </Button>
        <Button
          className="pickup-display-fullscreen"
          type="text"
          icon={fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
          onClick={() => void toggleFullscreen()}
        >
          {fullscreen ? '退出全屏' : '全屏显示'}
        </Button>
      </header>

      <div className="pickup-display-columns">
        <PickupColumn kind="preparing" orders={preparing.items} page={preparing.page} pages={preparing.pages} recentReady={recentReady} />
        <PickupColumn kind="ready" orders={ready.items} page={ready.page} pages={ready.pages} recentReady={recentReady} />
      </div>

      <footer className="pickup-display-footer">
        <span className={connectionError ? 'is-error' : ''}>
          {connectionError ? <ReloadOutlined spin /> : <WifiOutlined />}
          {connectionError ? `连接异常，正在重试：${connectionError}` : loading ? '正在连接订单系统' : '已连接 · 每 5 秒自动更新'}
        </span>
        <strong><ClockCircleOutlined /> 请留意屏幕与现场叫号，及时取餐</strong>
        <span>更新时间 {data.updatedAt ? data.updatedAt.slice(11, 16) : '--:--'}</span>
      </footer>
    </main>
  );
}
