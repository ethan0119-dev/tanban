import { DeleteOutlined, FolderOpenOutlined, PictureOutlined } from '@ant-design/icons';
import { Button, Image, Space, Typography } from 'antd';
import './image-picker.css';

interface ImagePickerFieldProps {
  value?: string;
  onChange?: (value: string) => void;
  onOpenLibrary: () => void;
  alt?: string;
  hint?: string;
  allowClear?: boolean;
}

export function ImagePickerField({
  value,
  onChange,
  onOpenLibrary,
  alt = '已选择图片',
  hint = '从商户图片库选择图片',
  allowClear = true,
}: ImagePickerFieldProps) {
  return (
    <div className="image-picker-field">
      <div className={`image-picker-field__preview ${value ? 'has-image' : ''}`}>
        {value
          ? <Image src={value} alt={alt} />
          : <div className="image-picker-field__empty"><PictureOutlined /><Typography.Text type="secondary">尚未选择图片</Typography.Text></div>}
      </div>
      <div className="image-picker-field__actions">
        <div><strong>{value ? '图片已选择' : '等待选择'}</strong><Typography.Text type="secondary">{hint}</Typography.Text></div>
        <Space>
          {value && allowClear && <Button type="text" danger icon={<DeleteOutlined />} onClick={() => onChange?.('')}>移除</Button>}
          <Button icon={<FolderOpenOutlined />} onClick={onOpenLibrary}>{value ? '重新选择' : '打开图片库'}</Button>
        </Space>
      </div>
    </div>
  );
}
