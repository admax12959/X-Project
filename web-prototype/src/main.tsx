// PROTOTYPE — layout prototype for docs/design/v1-scope.md「原型」. Throwaway; lives on branch prototype/layout.
// Question it answers: what should the v1 pages look like, especially the 分支谱系视图?
// Run: pnpm prototype (repo root). Fake data in memory; reload resets everything.
// Several whole-app design directions, switched from the floating bar (or Alt+←/→).
import { StrictMode, useLayoutEffect } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import { useLocation } from './router';
import { PrototypeBar, useDirectionKey, type Direction } from './prototype';
import { baseline } from './directions/baseline';
import { direction as workbench } from './directions/workbench';
import { direction as lifecycle } from './directions/lifecycle';
import { direction as releaselog } from './directions/releaselog';

const DIRECTIONS: Record<string, Direction> = { workbench, lifecycle, releaselog, baseline };

function App() {
  const { path } = useLocation();
  const key = useDirectionKey(Object.keys(DIRECTIONS));
  const D = DIRECTIONS[key];
  // On <html> so dialogs portalled into <body> still pick up the direction's scoped CSS.
  useLayoutEffect(() => {
    document.documentElement.dataset.direction = key;
  }, [key]);

  const detail = path.match(/^\/apps\/([^/]+)$/);
  const page =
    path === '/login' ? <D.Login /> :
    detail ? <D.AppDetail appKey={decodeURIComponent(detail[1])} /> :
    path === '/settings' ? <D.Settings /> :
    path === '/admin/platforms' ? <D.AdminPlatforms /> :
    path === '/admin/users' ? <D.AdminUsers /> :
    <D.AppList />;
  return (
    <>
      <div key={key}>{page}</div>
      <PrototypeBar directions={DIRECTIONS} current={key} />
    </>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
