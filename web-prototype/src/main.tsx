// PROTOTYPE — layout prototype for docs/design/v1-scope.md「原型」. Throwaway; lives on branch prototype/layout.
// Question it answers: what should the v1 pages look like, especially the 分支谱系视图?
// Run: pnpm prototype (repo root). Fake data in memory; reload resets everything.
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import { useLocation } from './router';
import { Login } from './pages/Login';
import { AppList } from './pages/AppList';
import { AppDetail } from './pages/AppDetail';
import { Settings } from './pages/Settings';
import { AdminPlatforms, AdminUsers } from './pages/Admin';

function App() {
  const { path } = useLocation();
  if (path === '/login') return <Login />;
  const detail = path.match(/^\/apps\/([^/]+)$/);
  if (detail) return <AppDetail appKey={decodeURIComponent(detail[1])} />;
  if (path === '/settings') return <Settings />;
  if (path === '/admin/platforms') return <AdminPlatforms />;
  if (path === '/admin/users') return <AdminUsers />;
  return <AppList />;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
