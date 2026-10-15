// Plan A · Console frame: narrow top bar + left navigation. Pages render inside <Outlet/>.
// Nav entries live here (Applications, Versions, Settings, and the admin group).
import { NavLink, Outlet, useNavigate } from 'react-router';
import { api } from '../api/client.ts';
import { useCurrentUser, useSession } from '../session/session.tsx';
import { LOCALES, setLocale, useT, type Locale, type MessageKey } from '../i18n/index.ts';
import { Button, Segmented } from '../ui/controls.tsx';

function NavItem({ to, label, end }: { to: string; label: MessageKey; end?: boolean }) {
  const { t } = useT();
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => `block whitespace-nowrap px-5 py-1.5 leading-5 hover:text-accent ${isActive ? 'font-bold text-accent' : 'text-text'}`}
    >
      {t(label)}
    </NavLink>
  );
}

/** `persist` (logged-in pages) also saves the choice on the user so it follows them to other devices. */
export function LanguageSwitch({ persist }: { persist?: boolean }) {
  const { t, locale } = useT();
  const { state, setUser } = useSession();
  const change = (l: Locale) => {
    setLocale(l);
    if (persist && state.status === 'authenticated') {
      const user = state.user;
      api.me.update({ language: l }).then(
        () => setUser({ ...user, language: l }),
        () => undefined, // the choice still applies in this browser
      );
    }
  };
  return <Segmented label={t('common.language')} value={locale} onChange={change} options={LOCALES.map((l) => ({ value: l, label: t(`lang.${l}`) }))} />;
}

export function Shell() {
  const { t } = useT();
  const user = useCurrentUser();
  const { logout } = useSession();
  const navigate = useNavigate();
  return (
    <div className="flex h-screen min-w-[1100px] flex-col">
      <header className="flex h-11 flex-none items-center gap-4 whitespace-nowrap border-b border-line bg-white px-4">
        <NavLink to="/" className="text-[15px] font-bold text-text">
          {t('app.brand')}
        </NavLink>
        <span className="flex-1" />
        <span className="text-[13px] text-text-2">
          <b className="font-semibold text-text">{user.displayName}</b>
          {user.isAdmin && <> · {t('term.administrator')}</>}
        </span>
        <LanguageSwitch persist />
        <Button
          small
          onClick={async () => {
            await logout();
            navigate('/login', { replace: true });
          }}
        >
          {t('shell.signOut')}
        </Button>
      </header>
      <div className="flex min-h-0 flex-1">
        <nav aria-label={t('shell.nav')} className="w-60 flex-none overflow-y-auto border-r border-line bg-white pb-20 pt-3">
          <NavItem to="/" end label="term.applications" />
          <NavItem to="/versions" label="term.versions" />
          <NavItem to="/settings"label="settings.title" />
          {user.isAdmin && (
            <>
              <div className="px-5 pb-1 pt-4 text-[13px] font-semibold text-text-3">{t('platforms.navGroup')}</div>
              <NavItem to="/admin/platforms" label="platforms.title" />
              <NavItem to="/admin/users" label="users.title" />
              <NavItem to="/admin/departments" label="departments.title" />
              <NavItem to="/admin/ldap" label="ldap.title" />
              <NavItem to="/admin/storage" label="storage.title" />
              <NavItem to="/admin/roles" label="roles.title" />
              <NavItem to="/admin/audit" label="audit.title" />
            </>
          )}
        </nav>
        <main className="min-w-0 flex-1 overflow-auto">
          <div className="max-w-[1500px] space-y-5 px-6 pb-24 pt-5">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
