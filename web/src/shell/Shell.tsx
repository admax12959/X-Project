// Plan A · Console frame: narrow top bar + left navigation. Pages render inside <Outlet/>.
// Later tickets add nav entries here (Versions #13, Settings #15, Administration #4/#5).
import { NavLink, Outlet, useNavigate } from 'react-router';
import { useCurrentUser, useSession } from '../session/session.tsx';
import { LOCALES, setLocale, useT, type MessageKey } from '../i18n/index.ts';
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

export function LanguageSwitch() {
  const { t, locale } = useT();
  return <Segmented label={t('common.language')} value={locale} onChange={setLocale} options={LOCALES.map((l) => ({ value: l, label: t(`lang.${l}`) }))} />;
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
        <LanguageSwitch />
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
          {user.isAdmin && <NavItem to="/admin/users" label="users.title" />}
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
