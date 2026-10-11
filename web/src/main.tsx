import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Link, Route, Routes } from 'react-router';
import './index.css';
import { useT } from './i18n/index.ts';
import { ApplicationsPage } from './pages/ApplicationsPage.tsx';
import { LoginPage } from './pages/LoginPage.tsx';
import { PlatformsPage } from './pages/PlatformsPage.tsx';
import { UsersPage } from './pages/UsersPage.tsx';
import { RequireSession, SessionProvider } from './session/session.tsx';
import { Shell } from './shell/Shell.tsx';

function NotFound() {
  const { t } = useT();
  return (
    <div className="space-y-2">
      <h1 className="text-2xl font-bold">{t('notFound.title')}</h1>
      <Link to="/" className="text-accent hover:underline">
        {t('notFound.back')}
      </Link>
    </div>
  );
}

// Routes: /login is public; everything else is inside RequireSession + Shell.
function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        element={
          <RequireSession>
            <Shell />
          </RequireSession>
        }
      >
        <Route index element={<ApplicationsPage />} />
        <Route path="admin/platforms" element={<PlatformsPage />} />
        <Route path="admin/users" element={<UsersPage />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <SessionProvider>
        <App />
      </SessionProvider>
    </BrowserRouter>
  </StrictMode>,
);
