import { BRAND, MAP, DEBOUNCE_MS } from './config.js';
import { listPosts, getPost, reportPost, MOCK } from './api.js';
import { initPostForm, isPicking, manageButton } from './postform.js';
import { initI18n, setLang, getLang, t, apply, formatDate } from './i18n.js';

const $ = (id) => document.getElementById(id);
const el = (tag, props = {}, ...kids) => {
  const e = Object.assign(document.createElement(tag), props);
  e.append(...kids.filter((k) => k != null));
  return e;
};

const GLYPHS = 'https://demotiles.maplibre.org/font/{fontstack}/{range}.pbf';
let map;
let posts = [];
let timer;
let controller;

function buildStyle() {
  if (MAP.pmtilesUrl) {
    // Hosted tiles: register the pmtiles protocol; a vector style must be supplied via config in that case.
    const protocol = new pmtiles.Protocol();
    maplibregl.addProtocol('pmtiles', protocol.tile);
    return {
      version: 8,
      glyphs: GLYPHS,
      sources: { base: { type: 'raster', url: `pmtiles://${MAP.pmtilesUrl}`, tileSize: 256 } },
      layers: [{ id: 'base', type: 'raster', source: 'base' }],
    };
  }
  return {
    version: 8,
    glyphs: GLYPHS,
    sources: { osm: { type: 'raster', tiles: MAP.fallbackTiles, tileSize: 256, attribution: MAP.fallbackAttribution, maxzoom: 19 } },
    layers: [{ id: 'osm', type: 'raster', source: 'osm' }],
  };
}

const geojson = () => ({
  type: 'FeatureCollection',
  features: posts.map((p) => ({
    type: 'Feature',
    properties: { id: p.id, kind: p.kind },
    geometry: { type: 'Point', coordinates: [p.last_seen_point.lon, p.last_seen_point.lat] },
  })),
});

function initMap() {
  map = new maplibregl.Map({ container: 'map', style: buildStyle(), center: MAP.center, zoom: MAP.zoom, maxBounds: MAP.maxBounds });
  map.addControl(new maplibregl.NavigationControl({ showCompass: false }));
  map.on('load', () => {
    map.addSource('posts', { type: 'geojson', data: geojson(), cluster: true, clusterRadius: 50, clusterMaxZoom: 14 });
    map.addLayer({ id: 'clusters', type: 'circle', source: 'posts', filter: ['has', 'point_count'],
      paint: { 'circle-color': '#1f5fa8', 'circle-radius': ['step', ['get', 'point_count'], 16, 10, 22, 50, 28], 'circle-stroke-width': 2, 'circle-stroke-color': '#fff' } });
    map.addLayer({ id: 'cluster-count', type: 'symbol', source: 'posts', filter: ['has', 'point_count'],
      layout: { 'text-field': ['get', 'point_count_abbreviated'], 'text-size': 13, 'text-font': ['Open Sans Regular'] }, paint: { 'text-color': '#fff' } });
    // Lost = red filled circle; found = green circle with a thick white ring (shape/ring differ, not only colour).
    map.addLayer({ id: 'lost', type: 'circle', source: 'posts', filter: ['all', ['!', ['has', 'point_count']], ['==', ['get', 'kind'], 'lost']],
      paint: { 'circle-color': '#d64545', 'circle-radius': 9, 'circle-stroke-width': 2, 'circle-stroke-color': '#fff' } });
    map.addLayer({ id: 'found', type: 'circle', source: 'posts', filter: ['all', ['!', ['has', 'point_count']], ['==', ['get', 'kind'], 'found']],
      paint: { 'circle-color': '#fff', 'circle-radius': 9, 'circle-stroke-width': 5, 'circle-stroke-color': '#2a8f5b' } });
    for (const l of ['lost', 'found']) {
      map.on('click', l, (e) => openDetail(e.features[0].properties.id));
      map.on('mouseenter', l, () => { map.getCanvas().style.cursor = 'pointer'; });
      map.on('mouseleave', l, () => { map.getCanvas().style.cursor = ''; });
    }
    map.on('click', 'clusters', async (e) => {
      const f = e.features[0];
      const zoom = await map.getSource('posts').getClusterExpansionZoom(f.properties.cluster_id);
      map.easeTo({ center: f.geometry.coordinates, zoom });
    });
    load();
  });
  map.on('moveend', scheduleLoad);
}

function scheduleLoad() {
  clearTimeout(timer);
  timer = setTimeout(load, DEBOUNCE_MS);
}

async function load() {
  controller?.abort();
  controller = new AbortController();
  const b = map.getBounds();
  const q = {
    bbox: [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()].map((n) => +n.toFixed(5)),
    kind: $('f-kind').value || undefined,
    species: $('f-species').value || undefined,
    sinceDays: +$('f-days').value || undefined,
  };
  $('status').textContent = t('loading');
  try {
    posts = await listPosts(q, controller.signal);
  } catch (err) {
    if (err.name === 'AbortError') return;
    posts = [];
    $('status').textContent = t('error');
    render();
    return;
  }
  render();
}

function render() {
  map.getSource('posts')?.setData(geojson());
  const list = $('list');
  list.replaceChildren();
  if (posts.length) $('status').textContent = t('count', { n: posts.length });
  else if ($('status').textContent !== t('error')) $('status').textContent = t('empty');
  for (const p of posts) {
    const photo = p.photos?.[0];
    const img = el('img', { src: photo?.url ?? '', alt: photo ? photoAlt(p) : t('no_photo'), loading: 'lazy' });
    const btn = el('button', { className: 'item', type: 'button' }, img,
      el('span', {}, el('b', {}, p.name || t('unnamed')),
        el('small', {}, `${t('kind.' + p.kind)} · ${t('species.' + p.species)} · ${formatDate(p.last_seen_at)}`)));
    btn.dataset.kind = p.kind;
    btn.addEventListener('click', () => {
      map.easeTo({ center: [p.last_seen_point.lon, p.last_seen_point.lat], zoom: Math.max(map.getZoom(), 15) });
      openDetail(p.id);
    });
    list.append(el('li', {}, btn));
  }
}

const photoAlt = (p) => t('photo.alt', { kind: t('kind.' + p.kind), species: t('species.' + p.species), date: formatDate(p.last_seen_at) });
const tri = (v) => t(v ?? 'unknown');

async function openDetail(id) {
  if (isPicking()) return;
  const p = posts.find((x) => x.id === id) ?? (await getPost(id));
  if (!p) return;
  const dlg = $('detail');
  const title = el('h2', { id: 'd-title' }, p.name || t('unnamed'), ' ', el('span', { className: `tag ${p.kind}` }, t('kind.' + p.kind)));
  const photos = el('div', { className: 'photos' }, ...(p.photos ?? []).map((ph) => el('img', { src: ph.url, alt: photoAlt(p) })));
  const row = (k, v) => (v ? [el('dt', {}, t(k)), el('dd', {}, v)] : []);
  const pt = p.last_seen_point;
  const dl = el('dl', {},
    ...row('species', t('species.' + p.species)),
    ...row('last_seen', formatDate(p.last_seen_at)),
    ...row('approx_location', `${pt.lat.toFixed(3)}, ${pt.lon.toFixed(3)}`),
    ...row('sex', t('sex.' + (p.sex ?? 'unknown'))),
    ...row('neutered', tri(p.neutered)),
    ...row('goes_outside', tri(p.goes_outside)),
    ...row('description', p.description),
    ...row('contact', p.contact));
  const reason = el('textarea', { rows: 3, maxLength: 500, id: 'r-reason' });
  const msg = el('p', { role: 'status' });
  const form = el('form', { hidden: true }, el('label', { htmlFor: 'r-reason' }, t('report.reason')), reason,
    el('div', { className: 'row' }, el('button', { type: 'submit' }, t('report.send'))), msg);
  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!reason.value.trim()) return;
    try { await reportPost(p.id, reason.value.trim()); msg.textContent = t('report.thanks'); }
    catch { msg.textContent = t('report.error'); }
  });
  const reportBtn = el('button', { type: 'button' }, t('report'));
  reportBtn.addEventListener('click', () => { form.hidden = false; reason.focus(); });
  const close = el('button', { type: 'button' }, t('close'));
  close.addEventListener('click', () => dlg.close());
  const manage = manageButton(p.id, () => dlg.close());
  dlg.replaceChildren(title, photos, dl, el('div', { className: 'row' }, reportBtn, manage, close), form);
  if (!dlg.open) dlg.showModal();
}

function init() {
  document.title = BRAND.appName;
  $('brand').textContent = BRAND.appName;
  $('mock-badge').hidden = !MOCK;
  $('lang').value = getLang();
  $('lang').addEventListener('change', (e) => setLang(e.target.value));
  document.addEventListener('langchange', () => { apply(); render(); if ($('detail').open) $('detail').close(); });
  for (const id of ['f-kind', 'f-species', 'f-days']) $(id).addEventListener('change', load);
  const setView = (v) => {
    document.body.dataset.view = v;
    $('tab-map').setAttribute('aria-pressed', v === 'map');
    $('tab-list').setAttribute('aria-pressed', v === 'list');
    if (v === 'map') map?.resize();
  };
  $('tab-map').addEventListener('click', () => setView('map'));
  $('tab-list').addEventListener('click', () => setView('list'));
  setView('map');
  initMap();
  initPostForm({ getMap: () => map, onChanged: () => load() });
}

await initI18n();
init();
