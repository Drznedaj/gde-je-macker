// Single place for brand strings and map settings.
export const BRAND = {
  appName: 'Gde je Macker',
  tagline: { 'sr-Latn': 'Mapa izgubljenih i pronađenih ljubimaca', 'sr-Cyrl': 'Мапа изгубљених и пронађених љубимаца', en: 'Map of lost and found pets' },
};

export const DEFAULT_LANG = 'sr-Latn';
export const LANGS = ['sr-Latn', 'sr-Cyrl', 'en'];

export const MAP = {
  center: [20.46, 44.8], // Belgrade [lon, lat]
  zoom: 11,
  maxBounds: [[18.5, 42.1], [23.1, 46.3]], // Serbia, with margin
  // Hosted vector tiles (PMTiles). Leave empty until tiles are hosted; the raster fallback is used then.
  pmtilesUrl: '',
  // Dev fallback: public OpenStreetMap raster tiles (light dev use only, see OSM tile policy).
  fallbackTiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
  fallbackAttribution: '© OpenStreetMap contributors',
};

// Rough Serbia box for the client-side pin check (the server does the exact check).
export const SERBIA_BOX = { south: 42.2, north: 46.2, west: 18.8, east: 23.05 };

export const API_BASE = '/api/v1';
export const DEBOUNCE_MS = 300;
