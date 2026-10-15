import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Link, Route, Routes } from 'react-router';
import './index.css';
import { useT } from './i18n/index.ts';
import { ApplicationHistory } from './pages/ApplicationHistory.tsx';
import { ApplicationLineage } from './pages/ApplicationLineage.tsx';
import { ApplicationOverview } from './pages/ApplicationOverview.tsx';
import { ApplicationPage } from './pages/ApplicationPage.tsx';
import { ApplicationsPage } from './pages/ApplicationsPage.tsx';
import { ApplicationVersions } from './pages/ApplicationVersions.tsx';
import { AuditLogPage } from './pages/AuditLogPage.tsx';
import { DepartmentsPage } from './pages/DepartmentsPage.tsx';
import { LdapConfigPage } from './pages/LdapConfigPage.tsx';
import { MailConfigPage } from './pages/MailConfigPage.tsx';
import { StorageConfigPage } from './pages/StorageConfigPage.tsx';
import { LoginPage } from './pages/LoginPage.tsx';
import { PlatformsPage } from './pages/PlatformsPage.tsx';
import { RolesPage } from './pages/RolesPage.tsx';
import { SettingsPage } from './pages/SettingsPage.tsx';
import { UsersPage } from './pages/UsersPage.tsx';
import { VersionsPage } from './pages/VersionsPage.tsx';
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
        <Route path="apps/:key" element={<ApplicationPage />}>
          <Route index element={<ApplicationOverview />} />
          <Route path="lineage" element={<ApplicationLineage />} />
          <Route path="versions" element={<ApplicationVersions />} />
          <Route path="history" element={<ApplicationHistory />} />
        </Route>
        <Route path="versions" element={<VersionsPage />} />
        <Route path="settings"element={<SettingsPage />} />
        <Route path="admin/platforms" element={<PlatformsPage />} />
        <Route path="admin/users" element={<UsersPage />} />
        <Route path="admin/departments" element={<DepartmentsPage />} />
        <Route path="admin/ldap" element={<LdapConfigPage />} />
        <Route path="admin/mail" element={<MailConfigPage />} />
        <Route path="admin/storage" element={<StorageConfigPage />} />
        <Route path="admin/roles" element={<RolesPage />} />
        <Route path="admin/audit" element={<AuditLogPage />} />
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
