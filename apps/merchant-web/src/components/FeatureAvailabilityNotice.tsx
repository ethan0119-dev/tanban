import { Alert, type AlertProps } from 'antd';
import { merchantFeatureCopy, type MerchantFeatureKey } from '../features/availability/copy';

type FeatureAvailabilityNoticeProps = Omit<AlertProps, 'message' | 'description'> & {
  feature: MerchantFeatureKey;
};

/**
 * Merchant-facing availability copy must come from the registry so that a
 * disabled capability is described consistently across menus and settings.
 */
export function FeatureAvailabilityNotice({ feature, type = 'info', showIcon = true, ...props }: FeatureAvailabilityNoticeProps) {
  const copy = merchantFeatureCopy[feature];
  return <Alert {...props} type={type} showIcon={showIcon} message={copy.title} description={copy.description} />;
}
