import type { ReactNode } from 'react';
import { can, useStore } from '../../data';
import { navigate, useLocation } from '../../router';

function NavLink({ to, children, match }: { to: string; children: ReactNode; match?: (p: string) => boolean }) {
  const { path } = useLocation();
  const active = match ? match(path) : path === to;
  return (
    <a
      href={to}
      onClick={(e) => (e.preventDefault(), navigate(to))}
      className={`block rounded-sm px-3 py-1.5 ${active ? 'bg-accent-wash font-medium text-accent' : 'text-ink hover:bg-wash'}`}
    >
      {children}
    </a>
  );
}

export function Shell({ title, children }: { title: ReactNode; children: ReactNode }) {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-52 shrink-0 flex-col border-r border-line bg-paper">
        <div className="flex h-12 items-center border-b border-line px-4 font-semibold">应用维护工具</div>
        <nav className="flex-1 space-y-0.5 p-2">
          <NavLink to="/apps" match={(p) => p.startsWith('/apps')}>应用</NavLink>
          <NavLink to="/settings">个人设置</NavLink>
          {can(s.role).admin && (
            <>
              <div className="px-3 pt-5 pb-1 text-[12px] text-ink-3">管理后台</div>
              <NavLink to="/admin/platforms">代码平台</NavLink>
              <NavLink to="/admin/users">用户</NavLink>
            </>
          )}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-10 flex h-12 items-center justify-between border-b border-line bg-paper px-6">
          <div className="min-w-0 truncate text-ink-2">{title}</div>
          <div className="flex items-center gap-4 text-[13px]">
            <span>
              {me.displayName}
              <span className="ml-1.5 text-ink-3">{me.dept}</span>
            </span>
            <button className="text-ink-2 hover:text-ink" onClick={() => navigate('/login')}>退出</button>
          </div>
        </header>
        <main className="flex-1 px-8 pt-7 pb-28">{children}</main>
      </div>
    </div>
  );
}
