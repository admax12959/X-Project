import type { ReactNode } from 'react';
import { can, TODAY, useStore } from '../../data';
import { navigate, useLocation } from '../../router';
import { LinkButton } from './ui';

const WEEK = ['日', '一', '二', '三', '四', '五', '六'];

function NavItem({ to, current, children }: { to: string; current: boolean; children: ReactNode }) {
  return (
    <a href={to} onClick={(e) => (e.preventDefault(), navigate(to))} aria-current={current ? 'page' : undefined} className="rl-nav-item">
      {children}
    </a>
  );
}

export type Crumb = { label: ReactNode; to?: string };

export function Shell({ crumbs, children }: { crumbs: Crumb[]; children: ReactNode }) {
  const s = useStore();
  const { path } = useLocation();
  const me = s.users.find((u) => u.username === s.me)!;
  const mine = s.apps.filter((a) => a.owners.includes(s.me) || a.pes.includes(s.me));
  const admin = can(s.role).admin;
  const today = `${TODAY} 周${WEEK[new Date(TODAY).getUTCDay()]}`;

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-44 shrink-0 flex-col rl-bg-2 lg:w-52" style={{ borderRight: '1px solid var(--rl-rule)' }}>
        <div className="flex h-14 items-center px-5">
          <a href="/apps" onClick={(e) => (e.preventDefault(), navigate('/apps'))} className="rl-t-md font-semibold rl-ink">
            应用维护工具
          </a>
        </div>
        <nav aria-label="主导航" className="flex-1 overflow-y-auto px-2 pt-2 pb-6">
          <NavItem to="/apps" current={path === '/apps' || path === '/'}>应用</NavItem>
          {mine.length > 0 && (
            <div className="mt-1 mb-3 pl-4">
              <div className="rl-label px-3 py-2">我是 Owner / PE</div>
              {mine.map((a) => (
                <NavItem key={a.key} to={`/apps/${a.key}`} current={path === `/apps/${a.key}`}>
                  <span className="min-w-0 truncate">{a.name}</span>
                </NavItem>
              ))}
            </div>
          )}
          <NavItem to="/settings" current={path === '/settings'}>个人设置</NavItem>
          {admin && (
            <>
              <div className="rl-label mt-6 px-3 py-2">管理后台</div>
              <NavItem to="/admin/platforms" current={path === '/admin/platforms'}>代码平台</NavItem>
              <NavItem to="/admin/users" current={path === '/admin/users'}>用户</NavItem>
            </>
          )}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex h-14 items-center gap-6 px-8" style={{ background: 'var(--rl-paper)', borderBottom: '1px solid var(--rl-rule)' }}>
          <nav aria-label="位置" className="flex min-w-0 flex-1 items-center gap-2">
            {crumbs.map((c, i) => (
              <span key={i} className="flex min-w-0 items-center gap-2">
                {i > 0 && <span className="rl-ink-3" aria-hidden>/</span>}
                {c.to ? (
                  <a href={c.to} onClick={(e) => (e.preventDefault(), navigate(c.to!))} className="rl-link shrink-0" style={{ color: 'var(--rl-ink-2)' }}>
                    {c.label}
                  </a>
                ) : (
                  <span className="min-w-0 truncate font-semibold rl-ink">{c.label}</span>
                )}
              </span>
            ))}
          </nav>
          <span className="hidden shrink-0 rl-ink-3 lg:inline" title="到期天数按这一天计算">今天 {today}</span>
          <span className="shrink-0">
            <span className="font-medium">{me.displayName}</span>
            <span className="ml-2 rl-ink-3">{me.dept}</span>
          </span>
          <LinkButton className="shrink-0" onClick={() => navigate('/login')}>退出</LinkButton>
        </header>
        <main className="min-w-0 flex-1 pb-28">{children}</main>
      </div>
    </div>
  );
}
