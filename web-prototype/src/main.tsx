// PROTOTYPE — layout prototype for docs/design/v1-scope.md「原型」. Throwaway; lives on branch prototype/layout.
// Question it answers: what should the v1 pages look like, especially the 分支谱系视图?
// Run: pnpm prototype (repo root). Fake data in memory; reload resets everything.
// Whole-app design directions, switched from the floating bar (or Alt+←/→). Languages: en / zh (floating bar, and each direction's Settings).
import { Component, StrictMode, Suspense, use, useLayoutEffect, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import './locales/core';
import { TABS, useLocation, type Tab } from './router';
import { PrototypeBar, useDirectionKey, type Direction } from './prototype';

// Lazy per direction: a half-written direction cannot break the others.
const LOADERS: Record<string, () => Promise<{ direction: Direction }>> = {
  console: () => import('./directions/console'),
  catalog: () => import('./directions/catalog'),
};
const NAMES: Record<string, string> = { console: 'A · Console', catalog: 'B · Catalog' };
const cache = new Map<string, Promise<Direction>>();
function load(key: string) {
  if (!cache.has(key)) cache.set(key, LOADERS[key]().then((m) => m.direction));
  return cache.get(key)!;
}

class Boundary extends Component<{ children: ReactNode }, { error?: Error }> {
  state: { error?: Error } = {};
  static getDerivedStateFromError(error: Error) {
    return { error };
  }
  render() {
    return this.state.error ? <pre style={{ padding: 24, whiteSpace: 'pre-wrap' }}>{String(this.state.error.stack ?? this.state.error)}</pre> : this.props.children;
  }
}

function Routed({ dirKey, path }: { dirKey: string; path: string }) {
  const D = use(load(dirKey));
  const detail = path.match(/^\/apps\/([^/]+)(?:\/([^/]+))?$/);
  const tab = (detail && (TABS as string[]).includes(detail[2]) ? detail[2] : 'overview') as Tab;
  return (
    path === '/login' ? <D.Login /> :
    detail ? <D.AppDetail appKey={decodeURIComponent(detail[1])} tab={tab} /> :
    path === '/versions' ? <D.Versions /> :
    path === '/settings' ? <D.Settings /> :
    path === '/admin/platforms' ? <D.AdminPlatforms /> :
    path === '/admin/users' ? <D.AdminUsers /> :
    <D.AppList />
  );
}

function App() {
  const { path } = useLocation();
  const key = useDirectionKey(Object.keys(LOADERS));
  // On <html> so dialogs portalled into <body> still pick up the direction's scoped CSS.
  useLayoutEffect(() => {
    document.documentElement.dataset.direction = key;
  }, [key]);
  return (
    <>
      <Boundary key={key}>
        <Suspense fallback={null}>
          <Routed dirKey={key} path={path} />
        </Suspense>
      </Boundary>
      <PrototypeBar names={NAMES} current={key} />
    </>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
