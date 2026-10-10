import type { ReactNode } from 'react';
import { can, TODAY, useStore } from '../../data';
import { navigate, useLocation } from '../../router';

function NavLink({ to, children, match }: { to: string; children: ReactNode; match?: (p: string) => boolean }) {
  const { path } = useLocation();
  const active = match ? match(path) : path === to;
  return (
    <a href={to} aria-current={active ? 'page' : undefined} onClick={(e) => (e.preventDefault(), navigate(to))}>
      {children}
    </a>
  );
}

export function go(to: string) {
  return (e: { preventDefault: () => void }) => (e.preventDefault(), navigate(to));
}

/** Left nav + top bar. The top bar carries where you are and today's date: every page here is read against today. */
export function Shell({ crumbs, children }: { crumbs: ReactNode; children: ReactNode }) {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const roleLabel = { admin: '管理员', owner: 'Owner', pe: 'PE' }[s.role];
  return (
    <div className="flex min-h-screen bg-[var(--lc-paper)] text-[var(--lc-ink)]">
      <aside className="sticky top-0 flex h-screen w-[200px] shrink-0 flex-col border-r border-[var(--lc-ink)] max-[1100px]:w-[168px]">
        <a href="/apps" onClick={go('/apps')} className="flex h-14 items-center gap-2 border-b border-[var(--lc-ink)] px-4 text-[length:var(--lc-text-md)] leading-none font-extrabold tracking-tight">
          <span aria-hidden className="inline-block h-3 w-3 bg-[var(--lc-ink)]" />
          应用维护
        </a>
        <nav className="lc-nav flex-1 pt-4" aria-label="主导航">
          <NavLink to="/apps" match={(p) => p === '/' || p.startsWith('/apps')}>应用</NavLink>
          <NavLink to="/settings">个人设置</NavLink>
          {can(s.role).admin && (
            <>
              <div className="lc-label px-4 pt-8 pb-2">管理后台</div>
              <NavLink to="/admin/platforms">代码平台</NavLink>
              <NavLink to="/admin/users">用户</NavLink>
            </>
          )}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 items-center gap-6 border-b border-[var(--lc-ink)] bg-[var(--lc-paper)] px-8 max-[1100px]:px-5">
          <div className="min-w-0 flex-1 truncate text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">{crumbs}</div>
          <div className="flex shrink-0 items-baseline gap-2 text-[length:var(--lc-text-sm)]">
            <span className="lc-label">今天</span>
            <span className="lc-num font-semibold text-[var(--lc-signal)]">{TODAY}</span>
          </div>
          <div className="flex shrink-0 items-center gap-4 border-l border-[var(--lc-rule)] pl-6 text-[length:var(--lc-text-sm)]">
            <span>
              <span className="font-semibold">{me.displayName}</span>
              <span className="ml-2 text-[var(--lc-muted)]">{roleLabel}</span>
            </span>
            <button type="button" className="lc-btn lc-btn--ghost !h-8" onClick={() => navigate('/login')}>
              退出
            </button>
          </div>
        </header>
        <main className="min-w-0 flex-1 px-8 pt-8 pb-32 max-[1100px]:px-5">{children}</main>
      </div>
    </div>
  );
}

export function Crumbs({ items }: { items: { label: ReactNode; to?: string }[] }) {
  return (
    <nav aria-label="位置" className="flex items-center gap-2">
      {items.map((it, i) => (
        <span key={i} className="flex items-center gap-2">
          {i > 0 && <span aria-hidden className="text-[var(--lc-rule-2)]">/</span>}
          {it.to ? (
            <a href={it.to} onClick={go(it.to)} className="lc-link">{it.label}</a>
          ) : (
            <span className="font-semibold text-[var(--lc-ink)]">{it.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}

export function PageHead({ title, sub, actions }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
      <div className="min-w-0">
        <h1 className="lc-display">{title}</h1>
        {sub && <div className="mt-3 max-w-[50rem] text-pretty text-[var(--lc-ink-2)]">{sub}</div>}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

/** Section head: a 2px ink rule with the head butted against it. */
export function SectionHead({ children, actions, id }: { children: ReactNode; actions?: ReactNode; id?: string }) {
  return (
    <div className="mb-4 flex min-h-10 items-end justify-between gap-4 border-t-2 border-[var(--lc-ink)] pt-3">
      <h2 id={id} className="lc-h2">{children}</h2>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </div>
  );
}

export function NoAccess({ what }: { what: string }) {
  return (
    <div className="max-w-[60ch] border-t-2 border-[var(--lc-ink)] pt-4">
      <h1 className="lc-h2">{what}只对管理员开放</h1>
      <p className="mt-2 text-[var(--lc-ink-2)]">当前视角不是管理员。用底部原型栏切换到管理员视角后再打开。</p>
      <a href="/apps" onClick={go('/apps')} className="lc-link mt-4 inline-block">回到应用</a>
    </div>
  );
}
