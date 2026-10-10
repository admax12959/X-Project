import { useState } from 'react';
import { appLines, can, daysUntil, isExpiring, lineVersions, useStore, userName } from '../../../data';
import { navigate } from '../../../router';
import { Shell } from '../Shell';
import { ExpiryText, Input, Mono, PageHeader, SectionTitle, Table } from '../ui';

export function AppList() {
  const s = useStore();
  const [q, setQ] = useState('');
  const [mine, setMine] = useState(false);
  const isMine = (a: (typeof s.apps)[number]) => a.owners.includes(s.me) || a.pes.includes(s.me);

  const apps = s.apps.filter((a) => (!mine || isMine(a)) && (q === '' || `${a.key} ${a.name} ${a.description}`.toLowerCase().includes(q.toLowerCase())));

  // Expiring lines of apps I'm on (admin sees all)
  const expiring = s.lines
    .filter(isExpiring)
    .map((l) => ({ line: l, app: s.apps.find((a) => a.key === l.appKey)! }))
    .filter(({ app }) => can(s.role).admin || isMine(app))
    .sort((a, b) => daysUntil(a.line.plannedEnd!) - daysUntil(b.line.plannedEnd!));

  const open = (key: string, line?: string) => navigate(`/apps/${key}${line ? `?line=${encodeURIComponent(line)}` : ''}`);

  return (
    <Shell title="应用">
      <PageHeader title="应用" />

      {expiring.length > 0 && (
        <section className="mb-10">
          <SectionTitle>即将到期的维护线</SectionTitle>
          <ul className="divide-y divide-line-2 border border-line bg-paper">
            {expiring.map(({ line, app }) => (
              <li key={line.id}>
                <button onClick={() => open(app.key, line.id)} className="grid w-full grid-cols-[minmax(0,2fr)_minmax(0,2fr)_minmax(0,2fr)_auto] items-baseline gap-4 px-4 py-2.5 text-left hover:bg-wash">
                  <span>
                    <span className="font-medium">{line.name}</span>
                    <span className="ml-2 text-ink-2">{line.status}</span>
                  </span>
                  <span className="truncate text-ink-2">{app.name}</span>
                  <Mono className="truncate text-ink-2">{line.branch}</Mono>
                  <ExpiryText line={line} />
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="mb-3 flex items-center gap-4">
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder="搜索应用名称或应用标识" className="max-w-xs" />
        <label className="flex items-center gap-1.5 text-[13px]">
          <input type="checkbox" checked={mine} onChange={(e) => setMine(e.target.checked)} />
          只看我负责的应用
        </label>
        <span className="ml-auto text-[13px] text-ink-3">{apps.length} 个应用</span>
      </div>
      <Table head={['应用', '仓库关联', 'Owner', 'PE', '维护线', '最新主线版本']}>
        {apps.map((a) => {
          const ls = appLines(s, a.key);
          const main = ls.find((l) => l.isMain)!;
          const maint = ls.filter((l) => !l.isMain);
          const live = maint.filter((l) => l.status !== '终止').length;
          const platform = s.platforms.find((p) => p.id === a.repo.platformId)!;
          return (
            <tr key={a.key} onClick={() => open(a.key)} className="cursor-pointer hover:bg-wash">
              <td>
                <div className="font-medium">{a.name}</div>
                <Mono className="text-ink-2">{a.key}</Mono>
              </td>
              <td>
                <Mono>{a.repo.path}</Mono>
                <div className="text-[12px] text-ink-3">{platform.type} · {new URL(platform.url).host}</div>
              </td>
              <td>{a.owners.map(userName).join('、')}</td>
              <td>{a.pes.length ? a.pes.map(userName).join('、') : <span className="text-ink-3">—</span>}</td>
              <td>
                {maint.length === 0 ? (
                  <span className="text-ink-3">无</span>
                ) : live === 0 ? (
                  <span className="text-ink-3">{maint.length} 条已终止</span>
                ) : (
                  <>
                    {live} 条在维护
                    {maint.length > live && <span className="text-ink-3">，{maint.length - live} 条已终止</span>}
                  </>
                )}
              </td>
              <td>
                {lineVersions(s, main.id).at(-1)?.number}
                <Mono className="ml-1.5 text-ink-3">{main.branch}</Mono>
              </td>
            </tr>
          );
        })}
      </Table>
    </Shell>
  );
}
