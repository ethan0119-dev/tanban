import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  formatPickupCodeForSpeech,
  normalizePickupDisplayLayout,
  PICKUP_SPEECH_PITCH,
  PICKUP_SPEECH_RATE,
  pickupAnnouncementText,
  PickupDisplayPage,
} from './PickupDisplayPage';

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({ user: { id: 1, name: '测试店主', storeName: '测试门店' } }),
}));

describe('pickup display', () => {
  afterEach(() => cleanup());

  it('normalizes explicit portrait and safe landscape defaults', () => {
    expect(normalizePickupDisplayLayout('portrait')).toBe('portrait');
    expect(normalizePickupDisplayLayout('anything')).toBe('landscape');
    expect(normalizePickupDisplayLayout(null)).toBe('landscape');
  });

  it('reads pickup codes digit by digit with a calm speaking profile', () => {
    expect(formatPickupCodeForSpeech('a012')).toBe('A 零 一 二');
    expect(pickupAnnouncementText('A012')).toBe('请取餐号 A 零 一 二 的顾客，前来取餐。');
    expect(PICKUP_SPEECH_RATE).toBeLessThan(1);
    expect(PICKUP_SPEECH_PITCH).toBeLessThanOrEqual(1);
  });

  it('renders large preparing and ready queues in portrait preview mode', () => {
    const view = render(
      <MemoryRouter initialEntries={['/__preview/pickup-display?layout=portrait']}>
        <PickupDisplayPage previewMode />
      </MemoryRouter>,
    );
    expect(view.container.querySelector('.pickup-display-page')?.getAttribute('data-layout')).toBe('portrait');
    expect(screen.getByRole('heading', { name: '码农咖啡鼓楼店' })).toBeTruthy();
    expect(screen.getByLabelText('取餐号 A012')).toBeTruthy();
    expect(screen.getByLabelText('取餐号 A008')).toBeTruthy();
    expect(screen.getByRole('button', { name: /全屏显示/ })).toBeTruthy();
  });
});
