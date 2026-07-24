export interface Coordinate {
  latitude: number;
  longitude: number;
}

const PI = Math.PI;
const A = 6378245;
const EE = 0.006693421622965943;

function outsideChina(latitude: number, longitude: number) {
  return longitude < 72.004 || longitude > 137.8347 || latitude < 0.8293 || latitude > 55.8271;
}

function transformLatitude(x: number, y: number) {
  let result = -100 + 2 * x + 3 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x));
  result += (20 * Math.sin(6 * x * PI) + 20 * Math.sin(2 * x * PI)) * 2 / 3;
  result += (20 * Math.sin(y * PI) + 40 * Math.sin(y / 3 * PI)) * 2 / 3;
  result += (160 * Math.sin(y / 12 * PI) + 320 * Math.sin(y * PI / 30)) * 2 / 3;
  return result;
}

function transformLongitude(x: number, y: number) {
  let result = 300 + x + 2 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x));
  result += (20 * Math.sin(6 * x * PI) + 20 * Math.sin(2 * x * PI)) * 2 / 3;
  result += (20 * Math.sin(x * PI) + 40 * Math.sin(x / 3 * PI)) * 2 / 3;
  result += (150 * Math.sin(x / 12 * PI) + 300 * Math.sin(x / 30 * PI)) * 2 / 3;
  return result;
}

/** Browser geolocation WGS-84 coordinate to the GCJ-02 coordinate used by WeChat and AMap. */
export function wgs84ToGcj02({ latitude, longitude }: Coordinate): Coordinate {
  if (outsideChina(latitude, longitude)) return { latitude, longitude };
  let deltaLatitude = transformLatitude(longitude - 105, latitude - 35);
  let deltaLongitude = transformLongitude(longitude - 105, latitude - 35);
  const radLatitude = latitude / 180 * PI;
  let magic = Math.sin(radLatitude);
  magic = 1 - EE * magic * magic;
  const sqrtMagic = Math.sqrt(magic);
  deltaLatitude = deltaLatitude * 180 / ((A * (1 - EE)) / (magic * sqrtMagic) * PI);
  deltaLongitude = deltaLongitude * 180 / (A / sqrtMagic * Math.cos(radLatitude) * PI);
  return { latitude: latitude + deltaLatitude, longitude: longitude + deltaLongitude };
}

/** Approximate inverse used only to place an existing GCJ-02 point on the OSM picker. */
export function gcj02ToWgs84(coordinate: Coordinate): Coordinate {
  const converted = wgs84ToGcj02(coordinate);
  return {
    latitude: coordinate.latitude * 2 - converted.latitude,
    longitude: coordinate.longitude * 2 - converted.longitude,
  };
}

export function roundedCoordinate(coordinate: Coordinate): Coordinate {
  return {
    latitude: Number(coordinate.latitude.toFixed(7)),
    longitude: Number(coordinate.longitude.toFixed(7)),
  };
}
