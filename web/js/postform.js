// Create-post form, pin picker, photo downscale, and manage-token flows (edit, renew, reunite).
// Privacy: the manage token lives only in localStorage on this device and in the link shown once.
// It is never logged, never sent anywhere except the X-Manage-Token header of the API call.
import { SERBIA_BOX } from './config.js';
import { createPost, uploadPhoto, updatePost, renewPost, reunitePost, getPost, ApiError } from './api.js';
import { t, formatDate } from './i18n.js';

const $ = (id) => document.getElementById(id);
const el = (tag, props = {}, ...kids) => {
  const e = Object.assign(document.createElement(tag), props);
  e.append(...kids.filter((k) => k != null));
  return e;
};

export const LIMITS = { name: 80, description: 2000, contactMin: 3, contact: 200, photos: 3 };
const MAX_SIDE = 1600;
const TOKENS_KEY = 'gjm.manage';

// ---- token storage (device only) ----
const readTokens = () => { try { return JSON.parse(localStorage.getItem(TOKENS_KEY)) ?? {}; } catch { return {}; } };
const writeTokens = (o) => localStorage.setItem(TOKENS_KEY, JSON.stringify(o));
export const getToken = (id) => readTokens()[id];
const saveToken = (id, token) => writeTokens({ ...readTokens(), [id]: token });
const forgetToken = (id) => { const o = readTokens(); delete o[id]; writeTokens(o); };
const manageLink = (id, token) => `${location.origin}${location.pathname}#manage=${encodeURIComponent(id)}.${encodeURIComponent(token)}`;

// ---- pure helpers (also used by tests) ----
export const inSerbia = ({ lat, lon }) =>
  lat >= SERBIA_BOX.south && lat <= SERBIA_BOX.north && lon >= SERBIA_BOX.west && lon <= SERBIA_BOX.east;

/** @returns {Record<string,string>} field -> message key (with vars) */
export function validate(v, now = Date.now()) {
  const errs = {};
  if (v.name.length > LIMITS.name) errs.name = ['err.max', { n: LIMITS.name }];
  if (v.description.length > LIMITS.description) errs.description = ['err.max', { n: LIMITS.description }];
  const c = v.contact.trim();
  if (c && (c.length < LIMITS.contactMin || c.length > LIMITS.contact)) errs.contact = ['err.contact_len', { min: LIMITS.contactMin, max: LIMITS.contact }];
  if (!v.point) errs.pin = ['err.pin_required', {}];
  else if (!inSerbia(v.point)) errs.pin = ['err.pin_outside', {}];
  if (!v.lastSeen || Number.isNaN(Date.parse(v.lastSeen))) errs.when = ['err.when_required', {}];
  else if (Date.parse(v.lastSeen) > now + 60e3) errs.when = ['err.when_future', {}];
  return errs;
}

/** Downscale to MAX_SIDE and re-encode as JPEG. Drawing on a canvas also drops EXIF client-side. */
export async function downscale(file) {
  const bmp = await createImageBitmap(file, { imageOrientation: 'from-image' });
  const k = Math.min(1, MAX_SIDE / Math.max(bmp.width, bmp.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.round(bmp.width * k);
  canvas.height = Math.round(bmp.height * k);
  canvas.getContext('2d').drawImage(bmp, 0, 0, canvas.width, canvas.height);
  bmp.close?.();
  return new Promise((res, rej) => canvas.toBlob((b) => (b ? res(b) : rej(new Error('encode'))), 'image/jpeg', 0.85));
}

// ---- state ----
let ctx;               // { getMap, onChanged }
let point = null;      // chosen last-seen point
let marker = null;
let photos = [];       // { blob, url }
let picking = false;
export const isPicking = () => picking;

const nowLocalInput = () => {
  const d = new Date(Date.now() - new Date().getTimezoneOffset() * 6e4);
  return d.toISOString().slice(0, 16);
};

export function initPostForm(context) {
  ctx = context;
  const form = $('post-form');
  $('new-post').addEventListener('click', openForm);
  $('pf-cancel').addEventListener('click', () => $('post-dlg').close());
  $('pin-btn').addEventListener('click', startPick);
  $('pick-ok').addEventListener('click', () => endPick(true));
  $('pick-cancel').addEventListener('click', () => endPick(false));
  $('photo-input').addEventListener('change', onFiles);
  form.addEventListener('submit', onSubmit);
  $('tk-copy').addEventListener('click', copyLink);
  $('tk-done').addEventListener('click', () => $('token-dlg').close());
  $('token-dlg').addEventListener('close', () => { $('tk-link').value = ''; $('tk-msg').textContent = ''; });
  document.addEventListener('langchange', () => { clearErrors(); renderPin(); renderPhotos(); $('manage-dlg').close(); $('token-dlg').close(); });
  window.addEventListener('hashchange', handleHash);
  handleHash();
}

function openForm() {
  if (!$('post-form').last_seen_at.value) $('post-form').last_seen_at.value = nowLocalInput();
  $('post-form').last_seen_at.max = nowLocalInput();
  renderPin();
  renderPhotos();
  if (!$('post-dlg').open) $('post-dlg').showModal();
}

// ---- pin picker (uses the main map) ----
let draft = null;
function startPick() {
  picking = true;
  draft = point;
  $('post-dlg').close();
  $('tab-map').click();
  $('pick-bar').hidden = false;
  $('pick-ok').disabled = !draft;
  $('pick-msg').textContent = t('pick.hint');
  ctx.getMap().on('click', onMapClick);
  ctx.getMap().getCanvas().style.cursor = 'crosshair';
}

function onMapClick(e) {
  const p = { lat: +e.lngLat.lat.toFixed(5), lon: +e.lngLat.lng.toFixed(5) };
  if (!inSerbia(p)) {
    $('pick-msg').textContent = t('err.pin_outside');
    return;
  }
  draft = p;
  $('pick-ok').disabled = false;
  $('pick-msg').textContent = t('pick.set', { lat: p.lat.toFixed(3), lon: p.lon.toFixed(3) });
  placeMarker(p);
}

function placeMarker(p) {
  const map = ctx.getMap();
  marker ??= new maplibregl.Marker({ color: '#d64545' });
  marker.setLngLat([p.lon, p.lat]).addTo(map);
}

function endPick(ok) {
  const map = ctx.getMap();
  map.off('click', onMapClick);
  map.getCanvas().style.cursor = '';
  $('pick-bar').hidden = true;
  if (ok) point = draft;
  if (point) placeMarker(point); else marker?.remove();
  if (!ok && !point) marker?.remove();
  // keep a click on a post marker from opening details in the same tick
  setTimeout(() => { picking = false; }, 0);
  openForm();
}

function renderPin() {
  $('pin-text').textContent = point ? t('pick.set', { lat: point.lat.toFixed(3), lon: point.lon.toFixed(3) }) : t('pick.none');
}

// ---- photos ----
async function onFiles(e) {
  const files = [...e.target.files];
  e.target.value = '';
  $('e-photos').textContent = '';
  for (const f of files) {
    if (photos.length >= LIMITS.photos) { $('e-photos').textContent = t('err.photos_max', { n: LIMITS.photos }); break; }
    if (!/^image\/(jpeg|png|webp)$/.test(f.type)) { $('e-photos').textContent = t('err.photo_type'); continue; }
    try {
      const blob = await downscale(f);
      photos.push({ blob, url: URL.createObjectURL(blob) });
    } catch { $('e-photos').textContent = t('err.photo_read'); }
  }
  renderPhotos();
}

function renderPhotos() {
  const list = $('photo-list');
  list.replaceChildren(...photos.map((p, i) => {
    const btn = (label, fn, disabled) => {
      const b = el('button', { type: 'button', textContent: label.sym, disabled });
      b.setAttribute('aria-label', t(label.key, { n: i + 1 }));
      b.addEventListener('click', fn);
      return b;
    };
    const move = (d) => () => { [photos[i], photos[i + d]] = [photos[i + d], photos[i]]; renderPhotos(); };
    return el('li', {},
      el('img', { src: p.url, alt: t('post.photo_n', { n: i + 1 }) }),
      el('div', { className: 'row' },
        btn({ sym: '←', key: 'photo.left' }, move(-1), i === 0),
        btn({ sym: '→', key: 'photo.right' }, move(1), i === photos.length - 1),
        btn({ sym: '×', key: 'photo.remove' }, () => { URL.revokeObjectURL(p.url); photos.splice(i, 1); renderPhotos(); })));
  }));
}

// ---- submit ----
const values = () => {
  const f = $('post-form');
  return {
    kind: f.kind.value, species: f.species.value, name: f.name.value.trim(), sex: f.sex.value,
    neutered: f.neutered.value, goes_outside: f.goes_outside.value, description: f.description.value.trim(),
    contact: f.contact.value.trim(), lastSeen: f.last_seen_at.value, point,
  };
};

function clearErrors() {
  for (const id of ['e-name', 'e-description', 'e-pin', 'e-when', 'e-contact', 'e-photos', 'pf-summary']) $(id).textContent = '';
  $('post-form').querySelectorAll('[aria-invalid]').forEach((n) => n.removeAttribute('aria-invalid'));
}

function showErrors(errs) {
  clearErrors();
  const map = { name: 'e-name', description: 'e-description', pin: 'e-pin', when: 'e-when', contact: 'e-contact' };
  const inputs = { name: 'name', description: 'description', when: 'last_seen_at', contact: 'contact' };
  for (const [k, [key, vars]] of Object.entries(errs)) {
    $(map[k]).textContent = t(key, vars);
    if (inputs[k]) $('post-form')[inputs[k]].setAttribute('aria-invalid', 'true');
  }
  $('pf-summary').textContent = t('err.fix');
  $('pf-summary').focus();
}

async function onSubmit(e) {
  e.preventDefault();
  const v = values();
  const errs = validate(v);
  if (Object.keys(errs).length) { showErrors(errs); return; }
  clearErrors();
  const btn = $('pf-submit');
  btn.disabled = true;
  btn.textContent = t('post.sending');
  try {
    const ids = [];
    for (const p of photos) ids.push((await uploadPhoto(p.blob)).id);
    const body = {
      kind: v.kind, species: v.species, sex: v.sex, neutered: v.neutered, goes_outside: v.goes_outside,
      last_seen_point: v.point, last_seen_at: new Date(v.lastSeen).toISOString(),
    };
    if (v.name) body.name = v.name;
    if (v.description) body.description = v.description;
    if (v.contact) body.contact = v.contact;
    if (ids.length) body.photo_ids = ids;
    const created = await createPost(body);
    saveToken(created.id, created.manage_token);
    showToken(created.id, created.manage_token);
    resetForm();
    $('post-dlg').close();
    ctx.onChanged(created);
  } catch (err) {
    const s = err instanceof ApiError ? err.status : 0;
    $('pf-summary').textContent = t(s === 429 ? 'err.rate' : s === 422 ? 'err.server_invalid' : s === 413 ? 'err.photo_big' : 'err.submit');
    $('pf-summary').focus();
  } finally {
    btn.disabled = false;
    btn.textContent = t('post.submit');
  }
}

function resetForm() {
  $('post-form').reset();
  photos.forEach((p) => URL.revokeObjectURL(p.url));
  photos = [];
  point = null;
  marker?.remove();
  renderPhotos();
  renderPin();
}

// ---- token shown once ----
function showToken(id, token) {
  $('tk-link').value = manageLink(id, token);
  $('token-dlg').showModal();
  $('tk-link').select();
}

async function copyLink() {
  const input = $('tk-link');
  try { await navigator.clipboard.writeText(input.value); $('tk-msg').textContent = t('token.copied'); }
  catch { input.select(); $('tk-msg').textContent = t('token.copy_manual'); }
}

// ---- manage flows ----
function handleHash() {
  const m = /^#manage=([^.]+)\.(.+)$/.exec(location.hash);
  if (!m) return;
  const id = decodeURIComponent(m[1]);
  saveToken(id, decodeURIComponent(m[2]));
  history.replaceState(null, '', location.pathname + location.search); // drop the token from the URL bar
  openManage(id);
}

/** Returns a "Manage this post" button when this device holds the token, else null. */
export function manageButton(id, closeFirst) {
  if (!getToken(id)) return null;
  const b = el('button', { type: 'button' }, t('manage.open'));
  b.addEventListener('click', () => { closeFirst?.(); openManage(id); });
  return b;
}

async function openManage(id) {
  const dlg = $('manage-dlg');
  const token = getToken(id);
  let p;
  try { p = await getPost(id); } catch { p = null; }
  if (!p || !token) {
    dlg.replaceChildren(el('h2', { id: 'mg-title' }, t('manage.title')), el('p', { role: 'alert' }, t('manage.gone')), closeBtn(dlg));
    if (!dlg.open) dlg.showModal();
    return;
  }
  const msg = el('p', { role: 'status', ariaLive: 'polite' });
  const say = (k, vars) => { msg.textContent = t(k, vars); };
  const desc = el('textarea', { id: 'mg-desc', rows: 4, maxLength: LIMITS.description, value: p.description ?? '' });
  const contact = el('input', { id: 'mg-contact', maxLength: LIMITS.contact, value: p.contact ?? '', autocomplete: 'off' });
  const guard = async (fn, okKey) => {
    try { await fn(); say(okKey); ctx.onChanged(); }
    catch (err) {
      if (err instanceof ApiError && err.status === 401) { forgetToken(id); say('manage.bad_token'); }
      else say('err.submit');
    }
  };
  const save = el('button', { type: 'button', className: 'primary' }, t('manage.save'));
  save.addEventListener('click', () => {
    const d = desc.value.trim();
    const c = contact.value.trim();
    if (d.length > LIMITS.description) return say('err.max', { n: LIMITS.description });
    if (c && (c.length < LIMITS.contactMin || c.length > LIMITS.contact)) return say('err.contact_len', { min: LIMITS.contactMin, max: LIMITS.contact });
    guard(() => updatePost(id, token, { description: d, contact: c }), 'manage.saved');
  });
  const renew = el('button', { type: 'button' }, t('manage.renew'));
  renew.addEventListener('click', () => guard(() => renewPost(id, token), 'manage.renewed'));
  const reunite = el('button', { type: 'button' }, t('manage.reunite'));
  reunite.addEventListener('click', () => {
    if (!confirm(t('manage.reunite_confirm'))) return;
    guard(async () => { await reunitePost(id, token); reunite.disabled = renew.disabled = save.disabled = true; }, 'manage.reunited');
  });
  const forget = el('button', { type: 'button' }, t('manage.forget'));
  forget.addEventListener('click', () => { forgetToken(id); dlg.close(); });
  dlg.replaceChildren(
    el('h2', { id: 'mg-title' }, `${t('manage.title')}: `, p.name || t('unnamed'), ' ', el('span', { className: `tag ${p.kind}` }, t('kind.' + p.kind))),
    el('p', {}, `${t('last_seen')}: ${formatDate(p.last_seen_at)} · ${t('manage.expires')}: ${formatDate(p.expires_at)}`),
    el('p', {}, el('small', {}, t('manage.photos_note'))),
    el('label', { htmlFor: 'mg-desc' }, t('description')), desc,
    el('label', { htmlFor: 'mg-contact' }, t('post.contact')), contact,
    el('small', {}, t('post.contact_public')),
    el('div', { className: 'row' }, save, renew, reunite),
    msg,
    el('div', { className: 'row' }, forget, closeBtn(dlg)));
  if (!dlg.open) dlg.showModal();
}

function closeBtn(dlg) {
  const b = el('button', { type: 'button' }, t('close'));
  b.addEventListener('click', () => dlg.close());
  return b;
}
