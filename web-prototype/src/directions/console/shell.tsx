import type { MouseEvent, ReactNode } from 'react';
import { can, useStore, userName } from '../../data';
import { setLocale, useT, type Locale } from '../../i18n';
import { navigate, useLocation } from '../../router';
import { LinkBtn } from './ui';

function NavLink({ to, active, children }: { to: string; active: boolean; children: ReactNode }) {
  return (
    <a
      href={to}
      aria-current={active ? 'page' : undefined}
      onClick={(e: MouseEvent) => {
        if (e.metaKey || e.ctrlKey) return;
        e.preventDefault();
        navigate(to);
      }}
    >
      {children}
    </a>
  );
}

export function Shell({ children, panel }: { children: ReactNode; panel?: ReactNode }) {
  const { t, tx, locale } = useT();
  const s = useStore();
  const { path } = useLocation();
  const admin = can(s.role).admin;
  const me = s.users.find((u) => u.username === s.me);
  return (
    <div className="cs-app">
      <header className="cs-topbar">
        <LinkBtn className="cs-brand" onClick={() => navigate('/')} style={{ color: 'var(--c-text)', fontWeight: 700 }}>
          {t('console.brand')}
        </LinkBtn>
        <span className="cs-spacer" />
        <span className="cs-user">
          <b>{me ? tx(me.displayName) : userName(s.me)}</b> · {t(`role.${s.role}`)}
        </span>
        <select aria-label={t('common.language')} className="cs-select cs-select-auto" style={{ height: 28, fontSize: 13 }} value={locale} onChange={(e) => setLocale(e.target.value as Locale)}>
          <option value="en">{t('lang.en')}</option>
          <option value="zh">{t('lang.zh')}</option>
        </select>
        <button type="button" className="cs-btn cs-btn-sm" onClick={() => navigate('/login')}>
          {t('common.signOut')}
        </button>
      </header>
      <div className="cs-body">
        <nav className="cs-nav" aria-label={t('console.nav')}>
          <NavLink to="/" active={path === '/' || path.startsWith('/apps')}>
            {t('term.applications')}
          </NavLink>
          <NavLink to="/versions" active={path === '/versions'}>
            {t('term.versions')}
          </NavLink>
          <NavLink to="/settings" active={path === '/settings'}>
            {t('console.settings')}
          </NavLink>
          {admin && (
            <>
              <div className="cs-nav-group">{t('console.administration')}</div>
              <NavLink to="/admin/platforms" active={path === '/admin/platforms'}>
                {t('term.codePlatforms')}
              </NavLink>
              <NavLink to="/admin/users" active={path === '/admin/users'}>
                {t('term.users')}
              </NavLink>
            </>
          )}
        </nav>
        <main className="cs-main">
          <div className="cs-scroll">
            <div className="cs-page">{children}</div>
          </div>
          {panel}
        </main>
      </div>
    </div>
  );
}
