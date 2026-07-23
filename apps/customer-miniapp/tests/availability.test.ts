import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  customerFeatureCopy,
  customerReleaseBlockers,
  customerSafeErrorMessage,
  showUnavailableFeature,
} from '../miniprogram/utils/availability';
import { ApiError } from '../miniprogram/utils/request';

const developerVocabulary = /(\bV\d+\b|\bMock\b|scene\s*=|联调|占位|对标系统|接口待接入|database|request:fail)/i;

describe('customer-facing availability copy', () => {
  beforeEach(() => {
    vi.stubGlobal('wx', { showModal: vi.fn() });
  });

  it('does not contain developer vocabulary', () => {
    for (const [key, copy] of Object.entries(customerFeatureCopy)) {
      expect(copy.title, key).toBeTruthy();
      expect(copy.content, key).toBeTruthy();
      expect(`${copy.title}${copy.content}`, key).not.toMatch(developerVocabulary);
    }
  });

  it('never exposes raw transport or database errors', () => {
    expect(customerSafeErrorMessage(new ApiError('database operation failed', 500, 'DB_ERROR'))).toBe('服务暂时繁忙，请稍后重试。');
    expect(customerSafeErrorMessage(new ApiError('request:fail url not in domain list', 0))).toBe('网络连接失败，请检查网络后重试。');
    expect(customerSafeErrorMessage(new ApiError('raw backend detail', 409, 'ORDER_NOT_PAYABLE'))).toBe('订单已关闭，请重新提交订单。');
  });

  it('shows registered copy for unavailable features', () => {
    showUnavailableFeature('DELIVERY');
    expect(wx.showModal).toHaveBeenCalledWith(expect.objectContaining({
      title: customerFeatureCopy.DELIVERY.title,
      content: customerFeatureCopy.DELIVERY.content,
    }));
  });

  it('tracks customer release blockers centrally', () => {
    expect(customerReleaseBlockers).toContain('STORED_VALUE');
    expect(customerReleaseBlockers).not.toContain('COUPON_REDEMPTION');
    expect(customerReleaseBlockers).toContain('MEMBERSHIP');
  });
});
