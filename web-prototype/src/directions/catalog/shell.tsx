import type { ReactNode } from 'react';
import { AppWindow, Layers, Server, Settings, Users } from 'lucide-react';
import { can, useStore, userName } from '../../data';
import { setLocale, useT, type Locale } from '../../i18n';
import { navigate, useLocation } from '../../router';
import { A, Empty, LinkBtn, Panel } from './ui';

export type Crumb = { label: string; to?: string };

export function Shell({ crumbs, children, adminOnly }: { crumbs: Crumb[]; children: ReactNode; adminOnly?: boolean }) {
  const { t, locale } = useT();
  const s = useStore();
  const { path } = useLocation();
  const admin = can(s.role).admin;
  const nav = (to: string, label: string, icon: ReactNode, current: boolean, dataCg: string) => (
    <A to={to} aria-current={current ? 'page' : undefined} data-cg={dataCg}>
      {icon}
      {label}
    </A>
  );
  return (
    <div className="cg-shell">
      <aside className="cg-side" aria-label={t('catalog.navLabel')}>
        <div className="cg-brand">{t('catalog.brand')}</div>
        <nav className="cg-nav">
          {nav('/', t('term.applications'), <AppWindow size={16} />, path === '/' || path.startsWith('/apps'), 'nav-apps')}
          {nav('/versions', t('term.versions'), <Layers size={16} />, path === '/versions', 'nav-versions')}
          {nav('/settings', t('catalog.settings'), <Settings size={16} />, path === '/settings', 'nav-settings')}
          {admin && (
            <>
              <div className="cg-nav-group">{t('catalog.administration')}</div>
              {nav('/admin/platforms', t('term.codePlatforms'), <Server size={16} />, path === '/admin/platforms', 'nav-platforms')}
              {nav('/admin/users', t('term.users'), <Users size={16} />, path === '/admin/users', 'nav-users')}
            </>
          )}
        </nav>
      </aside>
      <div style={{ minWidth: 0 }}>
        <header className="cg-top">
          <nav className="cg-crumbs" aria-label={t('catalog.breadcrumb')}>
            {crumbs.map((c, i) => (
              <span key={i} style={{ display: 'contents' }}>
                {i > 0 && (
                  <span className="cg-crumb-sep" aria-hidden>
                    /
                  </span>
                )}
                {c.to && i < crumbs.length - 1 ? <A to={c.to}>{c.label}</A> : <span aria-current={i === crumbs.length - 1 ? 'page' : undefined}>{c.label}</span>}
              </span>
            ))}
          </nav>
          <div className="cg-top-right">
            <label className="cg-lang">
              <span className="cg-grey">{t('common.language')}</span>
              <select value={locale} onChange={(e) => setLocale(e.target.value as Locale)} data-cg="lang" aria-label={t('common.language')}>
                <option value="en">{t('lang.en')}</option>
                <option value="zh">{t('lang.zh')}</option>
              </select>
            </label>
            <span className="cg-user" data-cg="user">
              <strong>{userName(s.me)}</strong> <span className="cg-grey">· {t(`role.${s.role}`)}</span>
            </span>
            <LinkBtn onClick={() => navigate('/login')} data-cg="signout">
              {t('common.signOut')}
            </LinkBtn>
          </div>
        </header>
        <main className="cg-page">{adminOnly && !admin ? <NoAccess /> : children}</main>
      </div>
    </div>
  );
}

function NoAccess() {
  const { t } = useT();
  return (
    <Panel bodyClass={null} data-cg="no-access">
      <Empty title={t('catalog.noAccess')}>
        <p>{t('catalog.noAccessBody')}</p>
      </Empty>
    </Panel>
  );
}
