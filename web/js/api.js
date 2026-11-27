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
