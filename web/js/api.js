// API client for docs/api/openapi.yaml. `?mock=1` (or localStorage mock=1) serves fixtures instead.
import { API_BASE } from './config.js';

const params = new URLSearchParams(location.search);
export const MOCK = params.get('mock') === '1';

let fixtureCache;
async function fixtures() {
  if (!fixtureCache) {
    const res = await fetch(new URL('../fixtures/posts.json', import.meta.url));
    const raw = await res.json();
    const now = Date.now();
    // Fixtures use days_ago so they never go stale.
    fixtureCache = raw.map(({ days_ago, ...p }) => {
      const seen = new Date(now - days_ago * 864e5).toISOString();
      return { ...p, last_seen_at: seen, created_at: seen, expires_at: new Date(now + (30 - days_ago) * 864e5).toISOString() };
    });
  }
  return fixtureCache;
}

/** @param {{bbox:number[], kind?:string, species?:string, sinceDays?:number, limit?:number}} q */
export async function listPosts({ bbox, kind, species, sinceDays, limit = 200 }, signal) {
  const since = sinceDays ? new Date(Date.now() - sinceDays * 864e5) : null;
  if (MOCK) {
    const [w, s, e, n] = bbox;
    return (await fixtures())
      .filter((p) => p.status === 'active')
      .filter((p) => p.last_seen_point.lon >= w && p.last_seen_point.lon <= e && p.last_seen_point.lat >= s && p.last_seen_point.lat <= n)
      .filter((p) => !kind || p.kind === kind)
      .filter((p) => !species || p.species === species)
      .filter((p) => !since || new Date(p.created_at) > since)
      .slice(0, limit);
  }
  const qs = new URLSearchParams({ bbox: bbox.join(','), limit: String(limit) });
  if (kind) qs.set('kind', kind);
  if (species) qs.set('species', species);
  if (since) qs.set('since', since.toISOString());
  const res = await fetch(`${API_BASE}/posts?${qs}`, { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function getPost(id) {
  if (MOCK) return (await fixtures()).find((p) => p.id === id);
  const res = await fetch(`${API_BASE}/posts/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

export async function reportPost(id, reason) {
  if (MOCK) return true;
  const res = await fetch(`${API_BASE}/posts/${encodeURIComponent(id)}/report`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ reason }),
  });
  if (res.status !== 202) throw new Error(`HTTP ${res.status}`);
  return true;
}

// ---- Create / manage (token goes in the X-Manage-Token header; never logged) ----
export class ApiError extends Error {
  constructor(status) { super(`HTTP ${status}`); this.status = status; }
}

const mockPhotos = new Map(); // photo id -> object URL (mock only)
const mockTokens = new Map(); // post id -> token (mock only)
const post = (id) => encodeURIComponent(id);
const jsonReq = (token, body, method = 'POST') => ({
  method,
  headers: { 'Content-Type': 'application/json', ...(token ? { 'X-Manage-Token': token } : {}) },
  body: body === undefined ? undefined : JSON.stringify(body),
});

async function mockManage(id, token) {
  const p = (await fixtures()).find((x) => x.id === id);
  if (!p) throw new ApiError(404);
  if (mockTokens.get(id) !== token) throw new ApiError(401);
  return p;
}

/** @param {Blob} blob already downscaled */
export async function uploadPhoto(blob) {
  if (MOCK) {
    const id = crypto.randomUUID();
    const url = URL.createObjectURL(blob);
    mockPhotos.set(id, url);
    return { id, url };
  }
  const fd = new FormData();
  fd.append('file', blob, 'photo.jpg');
  const res = await fetch(`${API_BASE}/photos`, { method: 'POST', body: fd });
  if (res.status !== 201) throw new ApiError(res.status);
  return res.json();
}

/** Returns the created post including manage_token (shown once). */
export async function createPost(body) {
  if (MOCK) {
    const { photo_ids, ...rest } = body;
    const now = Date.now();
    const p = { ...rest, id: crypto.randomUUID(), radius_m: 1000, status: 'active',
      photos: (photo_ids ?? []).map((id) => ({ id, url: mockPhotos.get(id) ?? '' })),
      created_at: new Date(now).toISOString(), expires_at: new Date(now + 30 * 864e5).toISOString() };
    (await fixtures()).push(p);
    const manage_token = [...crypto.getRandomValues(new Uint8Array(16))].map((b) => b.toString(16).padStart(2, '0')).join('');
    mockTokens.set(p.id, manage_token);
    return { ...p, manage_token };
  }
  const res = await fetch(`${API_BASE}/posts`, jsonReq(null, body));
  if (res.status !== 201) throw new ApiError(res.status);
  return res.json();
}

/** Only description, contact and photo_ids are editable. */
export async function updatePost(id, token, patch) {
  if (MOCK) {
    const p = await mockManage(id, token);
    const { photo_ids, ...rest } = patch;
    Object.assign(p, rest);
    if (photo_ids) p.photos = photo_ids.map((pid) => p.photos.find((x) => x.id === pid) ?? { id: pid, url: mockPhotos.get(pid) ?? '' });
    return;
  }
  const res = await fetch(`${API_BASE}/posts/${post(id)}`, jsonReq(token, patch, 'PATCH'));
  if (!res.ok) throw new ApiError(res.status);
}

export async function renewPost(id, token) {
  if (MOCK) {
    (await mockManage(id, token)).expires_at = new Date(Date.now() + 30 * 864e5).toISOString();
    return;
  }
  const res = await fetch(`${API_BASE}/posts/${post(id)}/renew`, jsonReq(token));
  if (!res.ok) throw new ApiError(res.status);
}

export async function reunitePost(id, token) {
  if (MOCK) { (await mockManage(id, token)).status = 'reunited'; return; }
  const res = await fetch(`${API_BASE}/posts/${post(id)}/reunite`, jsonReq(token, {}));
  if (!res.ok) throw new ApiError(res.status);
}
