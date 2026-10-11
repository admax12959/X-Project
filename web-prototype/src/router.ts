// Ten-line router: the prototype only needs pathname + search params.
// Not a decision about the real app's router.
import { useSyncExternalStore } from 'react';

const EVENT = 'proto:navigate';

function subscribe(cb: () => void) {
  window.addEventListener('popstate', cb);
  window.addEventListener(EVENT, cb);
  return () => {
    window.removeEventListener('popstate', cb);
    window.removeEventListener(EVENT, cb);
  };
}

export function useLocation() {
  const href = useSyncExternalStore(subscribe, () => location.pathname + location.search);
  const url = new URL(href, location.origin);
  return { path: url.pathname, params: url.searchParams };
}

export function navigate(to: string, { replace = false } = {}) {
  if (replace) history.replaceState(null, '', to);
  else history.pushState(null, '', to);
  window.dispatchEvent(new Event(EVENT));
}

export function setParam(key: string, value: string | null) {
  const url = new URL(location.href);
  if (value === null) url.searchParams.delete(key);
  else url.searchParams.set(key, value);
  navigate(url.pathname + url.search, { replace: true });
}

export type Tab = 'overview' | 'lineage' | 'versions' | 'history';
export const TABS: Tab[] = ['overview', 'lineage', 'versions', 'history'];
/** /apps/:key and /apps/:key/:tab. Every direction uses these four tabs so the directions stay comparable. */
export const appPath = (key: string, tab: Tab = 'overview') => (tab === 'overview' ? `/apps/${key}` : `/apps/${key}/${tab}`);
