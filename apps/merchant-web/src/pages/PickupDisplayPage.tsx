/* eslint-disable @next/next/no-img-element -- the store logo is merchant-managed runtime media */
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
export const PICKUP_SPEECH_RATE = 0.92;
export const PICKUP_SPEECH_PITCH = 0.98;

const spokenDigits: Record<string, string> = {
  '0': '零',
  '1': '一',
  '2': '二',
  '3': '三',
  '4': '四',
  '5': '五',
  '6': '六',
  '7': '七',
  '8': '八',
  '9': '九',
};

export function normalizePickupDisplayLayout(value: string | null | undefined): PickupDisplayLayout {
  return value === 'portrait' ? 'portrait' : 'landscape';
}

export function formatPickupCodeForSpeech(value: string): string {
  return value
    .trim()
    .toUpperCase()
    .split('')
    .map((character) => spokenDigits[character] ?? character)
    .join(' ');
}

export function pickupAnnouncementText(code: string): string {
  return `请取餐号 ${formatPickupCodeForSpeech(code)} 的顾客，前来取餐。`;
}

function loadVoiceEnabled(): boolean {
  try {
    return typeof localStorage !== 'undefined'
      && typeof localStorage.getItem === 'function'
      && localStorage.getItem(VOICE_STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

function saveVoiceEnabled(enabled: boolean): void {
  try {
    if (typeof localStorage !== 'undefined' && typeof localStorage.setItem === 'function') {
      localStorage.setItem(VOICE_STORAGE_KEY, enabled ? '1' : '0');
    }
  } catch {
    // 部分电视浏览器或隐私模式会禁用本地存储，不影响本次语音开关。
  }
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
  const [voiceEnabled, setVoiceEnabled] = useState(loadVoiceEnabled);
  const voiceEnabledRef = useRef(voiceEnabled);
  const voiceRef = useRef<SpeechSynthesisVoice | null>(null);
  const speechQueue = useRef<string[]>([]);
  const speaking = useRef(false);
  const speakNextRef = useRef<() => void>(() => {});

  const audioCtx = useRef<AudioContext | null>(null);

  const getAudioCtx = useCallback(() => {
    if (!audioCtx.current) audioCtx.current = new AudioContext();
    if (audioCtx.current.state === 'suspended') void audioCtx.current.resume();
    return audioCtx.current;
  }, []);

  const playChime = useCallback((): Promise<void> => {
    return new Promise((resolve) => {
      try {
        const ctx = getAudioCtx();
        const now = ctx.currentTime;
        // 柔和的下行叮咚，避免大屏或平板扬声器出现尖锐爆音。
        const tones: [number, number, number][] = [
          [784, 0, 0.32],
          [659, 0.24, 0.46],
        ];
        for (const [freq, delay, dur] of tones) {
          const osc = ctx.createOscillator();
          const gain = ctx.createGain();
          osc.type = 'sine';
          osc.frequency.value = freq;
          gain.gain.setValueAtTime(0.001, now + delay);
          gain.gain.linearRampToValueAtTime(0.09, now + delay + 0.025);
          gain.gain.exponentialRampToValueAtTime(0.001, now + delay + dur);
          osc.connect(gain).connect(ctx.destination);
          osc.start(now + delay);
          osc.stop(now + delay + dur);
        }
        setTimeout(resolve, 760);
      } catch { resolve(); }
    });
  }, [getAudioCtx]);

  const pickVoice = useCallback(() => {
    const voices = window.speechSynthesis?.getVoices() ?? [];
    const zhVoices = voices.filter((voice) => voice.lang.toLowerCase().startsWith('zh'));
    const preferredNaturalVoice =
      /xiaoxiao.*natural|xiaoyi.*natural|yunxi.*natural|yunyang.*natural|ting[- ]?ting|meijia|google.*(?:普通话|mandarin|中文)/i;
    voiceRef.current =
      zhVoices.find((voice) => preferredNaturalVoice.test(voice.name)) ??
      zhVoices.find((voice) => voice.lang.toLowerCase() === 'zh-cn' && voice.localService) ??
      zhVoices.find((voice) => voice.lang.toLowerCase() === 'zh-cn') ??
      zhVoices[0] ?? null;
  }, []);

  const configureUtterance = useCallback((utterance: SpeechSynthesisUtterance) => {
    utterance.lang = 'zh-CN';
    utterance.rate = PICKUP_SPEECH_RATE;
    utterance.pitch = PICKUP_SPEECH_PITCH;
    utterance.volume = 0.9;
    if (voiceRef.current) utterance.voice = voiceRef.current;
  }, []);

  const speakNext = useCallback(() => {
    if (!voiceEnabledRef.current || speaking.current || !speechQueue.current.length) return;
    const code = speechQueue.current.shift();
    if (!code) return;
    speaking.current = true;
    void playChime().then(() => {
      window.speechSynthesis?.resume();
      const utter = new SpeechSynthesisUtterance(pickupAnnouncementText(code));
      configureUtterance(utter);
      utter.onend = () => { speaking.current = false; speakNextRef.current(); };
      utter.onerror = () => { speaking.current = false; speakNextRef.current(); };
      window.speechSynthesis.speak(utter);
    });
  }, [configureUtterance, playChime]);

  useEffect(() => {
    speakNextRef.current = speakNext;
  }, [speakNext]);

  const speak = useCallback((codes: Set<string>) => {
    if (!voiceEnabledRef.current || !window.speechSynthesis) return;
    for (const code of codes) {
      speechQueue.current.push(code);
      speechQueue.current.push(code); // 播两次
    }
    speakNext();
  }, [speakNext]);

  const toggleVoice = useCallback(() => {
    setVoiceEnabled((prev) => {
      const next = !prev;
      voiceEnabledRef.current = next;
      saveVoiceEnabled(next);
      if (!next) { window.speechSynthesis?.cancel(); speechQueue.current = []; }
      else {
        window.speechSynthesis?.cancel();
        void playChime().then(() => {
          const test = new SpeechSynthesisUtterance('语音播报已开启。');
          configureUtterance(test);
          window.speechSynthesis?.speak(test);
        });
      }
      return next;
    });
  }, [configureUtterance, playChime]);

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
  }, [previewMode, speak]);

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
        <div className="pickup-display-actions">
          <Button
            type="text"
            icon={fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
            onClick={() => void toggleFullscreen()}
          >
            {fullscreen ? '退出全屏' : '全屏显示'}
          </Button>
          <Button
            type="text"
            icon={voiceEnabled ? <SoundOutlined /> : <MutedOutlined />}
            onClick={toggleVoice}
          />
        </div>
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
