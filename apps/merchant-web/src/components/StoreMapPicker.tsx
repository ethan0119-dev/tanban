import { EnvironmentOutlined } from '@ant-design/icons';
import { Alert, Typography } from 'antd';
import { CircleMarker, MapContainer, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import { useEffect } from 'react';
import type { Coordinate } from '../features/store/location';
import 'leaflet/dist/leaflet.css';

function MapCenter({ coordinate }: { coordinate: Coordinate }) {
  const map = useMap();
  useEffect(() => {
    map.setView([coordinate.latitude, coordinate.longitude], map.getZoom(), { animate: false });
  }, [coordinate.latitude, coordinate.longitude, map]);
  return null;
}

function PointSelector({ value, onChange }: { value: Coordinate; onChange: (coordinate: Coordinate) => void }) {
  useMapEvents({
    click(event) {
      onChange({ latitude: event.latlng.lat, longitude: event.latlng.lng });
    },
  });
  return <CircleMarker center={[value.latitude, value.longitude]} radius={10} pathOptions={{ color: '#ffffff', weight: 3, fillColor: '#167b5c', fillOpacity: 1 }} />;
}

export function StoreMapPicker({ value, address, onChange }: {
  value: Coordinate;
  address: string;
  onChange: (coordinate: Coordinate) => void;
}) {
  return <div className="store-map-picker">
    <Alert
      type="info"
      showIcon
      message="点击地图选择门店入口"
      description="可拖动和缩放地图，再点击门店实际入口。地图选点用于导航和距离判断，顾客看到的文字仍以“详细地址”为准。"
    />
    <div className="store-map-picker-address">
      <EnvironmentOutlined />
      <div><Typography.Text type="secondary">当前门店地址</Typography.Text><strong>{address || '尚未填写详细地址，请关闭后先填写'}</strong></div>
    </div>
    <MapContainer center={[value.latitude, value.longitude]} zoom={16} scrollWheelZoom className="store-map-picker-canvas">
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <MapCenter coordinate={value} />
      <PointSelector value={value} onChange={onChange} />
    </MapContainer>
  </div>;
}
