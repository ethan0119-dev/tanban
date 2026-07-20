import { describe, expect, it } from 'vitest';
import { cloneDecoration, DEFAULT_DECORATION } from './defaults';
import { normalizeConfig, toApiConfig } from './api';

describe('decoration hotspot image', () => {
  it('round-trips percentage hotspots and controlled actions', () => {
    const config = cloneDecoration(DEFAULT_DECORATION);
    config.homeModules.push({
      id: 'home-map',
      type: 'HOTSPOT_IMAGE',
      enabled: true,
      sortOrder: 90,
      title: '首页导航',
      subtitle: '',
      imageUrl: 'https://cdn.example.com/home.webp',
      hotspots: [
        { id: 'menu', x: 5.25, y: 20, width: 30, height: 12.5, label: '堂食点单', action: { type: 'OPEN_MENU' } },
        { id: 'phone', x: 60, y: 75, width: 35, height: 20, label: '联系门店', action: { type: 'CALL_PHONE', phone: '18600000000' } },
      ],
    });

    const normalized = normalizeConfig(toApiConfig(config));
    const hotspotModule = normalized.homeModules.find((item) => item.id === 'home-map');

    expect(hotspotModule).toMatchObject({ type: 'HOTSPOT_IMAGE', imageUrl: 'https://cdn.example.com/home.webp' });
    expect(hotspotModule?.hotspots).toEqual([
      { id: 'menu', x: 5.25, y: 20, width: 30, height: 12.5, label: '堂食点单', action: { type: 'OPEN_MENU' } },
      { id: 'phone', x: 60, y: 75, width: 35, height: 20, label: '联系门店', action: { type: 'CALL_PHONE', phone: '18600000000' } },
    ]);
  });

  it('clamps malformed hotspot geometry and drops unsafe actions', () => {
    const config = normalizeConfig({
      home: { modules: [{
        id: 'safe-hotspot',
        type: 'HOTSPOT_IMAGE',
        enabled: true,
        sortOrder: 10,
        config: {
          imageUrl: 'https://cdn.example.com/home.png',
          hotspots: [{ id: 'unsafe', x: 95, y: 98, width: 40, height: 30, action: { type: 'JAVASCRIPT', url: 'javascript:alert(1)' } }],
        },
      }] },
    });

    expect(config.homeModules[0].hotspots?.[0]).toMatchObject({ x: 95, y: 98, width: 5, height: 2, action: { type: 'NONE' } });
  });
});
