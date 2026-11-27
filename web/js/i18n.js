import { DEFAULT_LANG, LANGS } from './config.js';

const dicts = {};
let lang = DEFAULT_LANG;
const KEY = 'gjm.lang';

export async function initI18n() {
  const saved = localStorage.getItem(KEY);
  lang = LANGS.includes(saved) ? saved : DEFAULT_LANG;
  await Promise.all(LANGS.map(async (l) => {
    dicts[l] = await (await fetch(new URL(`../i18n/${l}.json`, import.meta.url))).json();
  }));
  apply();
}

export const getLang = () => lang;

export function setLang(l) {
  if (!LANGS.includes(l)) return;
  lang = l;
  localStorage.setItem(KEY, l);
  apply();
  document.dispatchEvent(new CustomEvent('langchange'));
}

export function t(key, vars = {}) {
  const s = dicts[lang]?.[key] ?? dicts.en?.[key] ?? key;
  return s.replace(/\{(\w+)\}/g, (_, k) => vars[k] ?? '');
}

export function apply(root = document) {
  document.documentElement.lang = lang === 'en' ? 'en' : lang === 'sr-Cyrl' ? 'sr-Cyrl' : 'sr-Latn';
  root.querySelectorAll('[data-i18n]').forEach((el) => { el.textContent = t(el.dataset.i18n); });
  root.querySelectorAll('[data-i18n-label]').forEach((el) => { el.setAttribute('aria-label', t(el.dataset.i18nLabel)); });
}

export function formatDate(iso) {
  const loc = lang === 'en' ? 'en-GB' : lang === 'sr-Cyrl' ? 'sr-Cyrl-RS' : 'sr-Latn-RS';
  return new Date(iso).toLocaleDateString(loc, { day: 'numeric', month: 'short', year: 'numeric' });
}
