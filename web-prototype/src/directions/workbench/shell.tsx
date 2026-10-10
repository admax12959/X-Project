import { useState, type ReactNode } from 'react';
import { can, isExpiring, lineageOrder, useStore, userName } from '../../data';
import { navigate, useLocation } from '../../router';
import { dueText, Mono, RailGlyph } from './ui';

// Expanded apps in the nav tree survive page changes (each page renders its own Shell).
const expanded = new Set<string>();

const ROLE_LABEL = { admin: '管理员', owner: 'Owner', pe: 'PE' } as const;

/** Top bar + tree nav. `children` fills the rest of the viewport; panes inside scroll on their own. */
export function Shell({ children }: { children: ReactNode }) {
  const s = useStore();
  return (
    <div className="flex h-screen flex-col">
      <header className="flex h-11 shrink-0 items-center justify-between border-b border-ink bg-paper px-4">
        <button type="button" onClick={() => navigate('/apps')} className="text-[length:var(--wb-text-md)] font-bold tracking-[0.02em]">
          应用维护
        </button>
        <div className="flex items-center gap-5 text-[length:var(--wb-text-sm)]">
          <span>
            <span className="font-semibold">{userName(s.me)}</span>
            <span className="ml-1.5 text-ink-3">{ROLE_LABEL[s.role]}</span>
          </span>
          <button type="button" className="wb-link" onClick={() => navigate('/settings')}>个人设置</button>
          <button type="button" className="wb-link" onClick={() => navigate('/login')}>退出登录</button>
        </div>
      </header>
      <div className="flex min-h-0 flex-1">
        <Nav />
        <div className="flex min-w-0 flex-1">{children}</div>
      </div>
    </div>
  );
}

function Nav() {
  const s = useStore();
  const { path, params } = useLocation();
  const [q, setQ] = useState('');
  const current = path.match(/^\/apps\/([^/]+)$/)?.[1];
  const currentKey = current ? decodeURIComponent(current) : undefined;
  const selLine = params.get('line');
  const [, force] = useState(0);
  if (currentKey && !expanded.has(currentKey)) expanded.add(currentKey);
  const toggle = (k: string) => (expanded.has(k) ? expanded.delete(k) : expanded.add(k), force((n) => n + 1));

  const expiringCount = s.lines.filter(isExpiring).length;
  const needle = q.trim().toLowerCase();
  const isAdmin = can(s.role).admin;

  const item = (active: boolean) =>
    `flex w-full items-center gap-2 rounded-[2px] px-2 py-[5px] text-left ${active ? 'bg-ink text-white' : 'hover:bg-[var(--wb-sel-row)]'}`;

  return (
    <nav className="wb-scroll flex w-[244px] shrink-0 flex-col overflow-y-auto border-r border-line bg-[var(--wb-nav)]" aria-label="导航">
      <div className="px-3 pt-3 pb-2">
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="筛选应用或维护线" className="wb-input h-7 bg-paper text-[length:var(--wb-text-sm)]" />
      </div>

      <div className="px-2">
        <button type="button" className={item(path === '/apps' || path === '/')} onClick={() => navigate('/apps')}>
          <span className="flex-1 font-semibold">全部应用</span>
          {expiringCount > 0 && <span className={`text-[length:var(--wb-text-xs)] font-semibold ${path === '/apps' || path === '/' ? 'text-white' : 'text-accent'}`}>{expiringCount} 条将到期</span>}
        </button>
      </div>

      <ul className="mt-1 flex-1 px-2 pb-4">
        {s.apps.map((a) => {
          const rows = lineageOrder(s, a.key);
          const appHit = !needle || a.key.includes(needle) || a.name.toLowerCase().includes(needle);
          const lineHits = rows.filter(({ line }) => !needle || line.name.toLowerCase().includes(needle) || line.branch.includes(needle));
          if (!appHit && lineHits.length === 0) return null;
          const open = !!needle || expanded.has(a.key);
          const shown = appHit && !needle ? rows : lineHits;
          const appActive = currentKey === a.key && !selLine;
          const expiring = rows.some(({ line }) => isExpiring(line));
          return (
            <li key={a.key} className="mt-0.5">
              <div className="flex items-stretch">
                <button type="button" aria-label={open ? `收起${a.name}` : `展开${a.name}`} aria-expanded={open} onClick={() => toggle(a.key)} className="w-5 shrink-0 text-ink-3 hover:text-ink">
                  <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden className={`mx-auto transition-transform duration-150 ${open ? 'rotate-90' : ''}`}>
                    <path d="M3 1.5 L7 5 L3 8.5" fill="none" stroke="currentColor" strokeWidth="1.6" />
                  </svg>
                </button>
                <button type="button" className={item(appActive)} onClick={() => navigate(`/apps/${a.key}`)}>
                  <span className="min-w-0 flex-1 truncate font-semibold">{a.name}</span>
                  {!open && expiring && <span className={`text-[length:var(--wb-text-xs)] font-semibold ${appActive ? 'text-white' : 'text-accent'}`}>将到期</span>}
                  <Mono className={`text-[length:var(--wb-text-xs)] ${appActive ? 'text-white/70' : 'text-ink-3'}`}>{a.key}</Mono>
                </button>
              </div>
              {open && (
                <ul className="mb-1.5 ml-5">
                  {shown.map(({ line, depth }) => {
                    const active = currentKey === a.key && selLine === line.id;
                    const due = isExpiring(line) ? dueText(line) : null;
                    return (
                      <li key={line.id}>
                        <button
                          type="button"
                          className={`${item(active)} py-[3px]`}
                          style={{ paddingLeft: 8 + depth * 12 }}
                          onClick={() => navigate(`/apps/${a.key}?line=${encodeURIComponent(line.id)}`)}
                          title={`${line.isMain ? '主线' : line.name} · ${line.status}`}
                        >
                          <span className={active ? '[&_line]:stroke-white' : ''}>
                            <RailGlyph status={line.status} main={line.isMain} width={14} />
                          </span>
                          <span className={`min-w-0 flex-1 truncate ${line.status === '终止' && !active ? 'text-ink-3' : ''}`}>{line.isMain ? '主线' : line.name}</span>
                          {due && <span className={`shrink-0 text-[length:var(--wb-text-xs)] font-semibold ${active ? 'text-white' : 'text-accent'}`}>{due.replace('后到期', '')}</span>}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              )}
            </li>
          );
        })}
      </ul>

      <div className="border-t border-line px-2 py-2">
        <button type="button" className={item(path === '/settings')} onClick={() => navigate('/settings')}>个人设置</button>
        {isAdmin && (
          <>
            <div className="mt-2 px-2 pb-1 text-[length:var(--wb-text-xs)] font-semibold text-ink-3">管理后台</div>
            <button type="button" className={item(path === '/admin/platforms')} onClick={() => navigate('/admin/platforms')}>代码平台</button>
            <button type="button" className={item(path === '/admin/users')} onClick={() => navigate('/admin/users')}>用户</button>
          </>
        )}
        <div className="h-12" />
      </div>
    </nav>
  );
}

/** Full-height scrolling content column for pages without an inspector. */
export function Page({ children }: { children: ReactNode }) {
  return (
    <main className="wb-scroll min-w-0 flex-1 overflow-y-auto">
      <div className="px-8 pt-6 pb-24">{children}</div>
    </main>
  );
}

export function PageTitle({ title, sub, actions }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-5 flex items-end justify-between gap-6">
      <div className="min-w-0">
        <h1 className="text-[length:var(--wb-text-xl)] leading-none font-bold tracking-[-0.01em]">{title}</h1>
        {sub && <div className="mt-2.5 max-w-[80ch] text-ink-2">{sub}</div>}
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </div>
  );
}
