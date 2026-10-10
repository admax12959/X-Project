import { useEffect, type ReactNode } from 'react';
import { actions, can, useStore, userName, type Role } from './data';
import { navigate, setParam, useLocation } from './router';
import { VARIANTS } from './lineage';

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
      <PrototypeBar />
    </div>
  );
}

const ROLES: { key: Role; label: string }[] = [
  { key: 'admin', label: '管理员' },
  { key: 'owner', label: 'Owner' },
  { key: 'pe', label: 'PE' },
];

/** PROTOTYPE ONLY. Not part of the design being evaluated. */
function PrototypeBar() {
  const s = useStore();
  const { path, params } = useLocation();
  const onDetail = /^\/apps\/[^/]+$/.test(path);
  const keys = Object.keys(VARIANTS);
  const current = keys.includes(params.get('variant') ?? '') ? params.get('variant')! : keys[0];
  const cycle = (d: number) => setParam('variant', keys[(keys.indexOf(current) + d + keys.length) % keys.length]);

  useEffect(() => {
    if (!onDetail) return;
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (t.closest('input, textarea, select, [contenteditable]') || document.querySelector('[role=dialog]')) return;
      if (e.key === 'ArrowLeft') cycle(-1);
      if (e.key === 'ArrowRight') cycle(1);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  if (!import.meta.env.DEV) return null;
  return (
    // deslop-ignore-next-line 19
    <div className="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-4 rounded-full bg-[#111] px-4 py-2 text-[13px] text-white shadow-lg">
      <span className="text-white/50">原型</span>
      <span className="flex gap-1">
        <span className="text-white/50">视角</span>
        {ROLES.map((r) => (
          <button key={r.key} onClick={() => actions.setRole(r.key)} className={`px-1.5 ${s.role === r.key ? 'font-semibold underline underline-offset-4' : 'text-white/70'}`}>
            {r.label}
          </button>
        ))}
      </span>
      {onDetail && (
        <span className="flex items-center gap-2 border-l border-white/20 pl-4">
          <button onClick={() => cycle(-1)} aria-label="上一个变体" className="px-1">←</button>
          <span className="min-w-40 text-center">
            谱系 {current} · {VARIANTS[current].name}
          </span>
          <button onClick={() => cycle(1)} aria-label="下一个变体" className="px-1">→</button>
        </span>
      )}
      <span className="text-white/40">{userName(s.me)}</span>
    </div>
  );
}
