import { useState } from 'react';
import { actions, can, childLines, isExpiring, lineById, lineVersions, useStore, userName, versionById, type Line, type LineStatus } from '../../data';
import { setParam, useLocation } from '../../router';
import { LifecycleChart } from './Chart';
import { DeleteAppDialog, EditAppDialog, ForkDialog, StatusDialog, VersionDialog } from './dialogs';
import { Crumbs, go, PageHead, SectionHead, Shell } from './Shell';
import { dueText, lineSolidEnd, lineStart, span } from './time';
import { Button, Code, Textarea } from './ui';

export function AppDetail({ appKey }: { appKey: string }) {
  const s = useStore();
  const { params } = useLocation();
  const [forkFrom, setForkFrom] = useState<string | null>(null);
  const [registerOn, setRegisterOn] = useState<Line | null>(null);
  const [statusTo, setStatusTo] = useState<{ line: Line; to: LineStatus } | null>(null);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const app = s.apps.find((a) => a.key === appKey);

  if (!app)
    return (
      <Shell crumbs={<Crumbs items={[{ label: '应用', to: '/apps' }, { label: appKey }]} />}>
        <div className="max-w-[60ch] border-t-2 border-[var(--lc-ink)] pt-4">
          <h1 className="lc-h2">没有应用标识为 <Code>{appKey}</Code> 的应用</h1>
          <p className="mt-2 text-[var(--lc-ink-2)]">它可能已被管理员删除。</p>
          <a href="/apps" onClick={go('/apps')} className="lc-link mt-4 inline-block">回到应用</a>
        </div>
      </Shell>
    );

  const perm = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const lineParam = params.get('line');
  const selected = lineParam && s.lines.some((l) => l.id === lineParam && l.appKey === app.key) ? lineParam : `${app.key}:main`;
  const selectedVer = params.get('ver');
  const select = (lineId: string, verId?: string) => {
    setParam('line', lineId);
    setParam('ver', verId ?? null);
  };
  const line = lineById(selected);
  const firstVersion = s.versions.find((v) => lineById(v.lineId)?.appKey === app.key);

  return (
    <Shell crumbs={<Crumbs items={[{ label: '应用', to: '/apps' }, { label: app.name }]} />}>
      <PageHead
        title={app.name}
        sub={
          <>
            <Code className="font-semibold text-[var(--lc-ink)]">{app.key}</Code>
            {app.description && <span className="ml-3">{app.description}</span>}
          </>
        }
        actions={
          <>
            {perm.admin && (
              <Button kind="danger" onClick={() => setDeleting(true)}>
                删除应用
              </Button>
            )}
            {perm.editApp && <Button onClick={() => setEditing(true)}>编辑应用</Button>}
            {perm.manageLines && (
              <Button kind="primary" onClick={() => setForkFrom('')} disabled={!firstVersion} title={firstVersion ? undefined : '还没有任何版本可以作为源版本'}>
                分叉维护线
              </Button>
            )}
          </>
        }
      />

      <LifecycleChart appKey={app.key} mainBranch={app.mainBranch} selected={selected} selectedVer={selectedVer} onSelect={select} />

      {line && (
        <LinePanel
          line={line}
          mainBranch={app.mainBranch}
          manage={perm.manageLines}
          selectedVer={selectedVer}
          onSelect={select}
          onFork={(vid) => setForkFrom(vid)}
          onRegister={() => setRegisterOn(line)}
          onStatus={(to) => setStatusTo({ line, to })}
        />
      )}

      <div className="mt-16 grid grid-cols-12 gap-x-10 gap-y-12">
        <section className="col-span-5 max-[1100px]:col-span-12" aria-labelledby="app-info">
          <SectionHead id="app-info">应用信息</SectionHead>
          <dl className="grid grid-cols-[96px_minmax(0,1fr)] gap-x-4 gap-y-3">
            <dt className="lc-muted">仓库关联</dt>
            <dd>
              <Code>{app.repo.path}</Code>
              <div className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                {platform?.type} {platform && new URL(platform.url).host} · 项目 ID <span className="lc-num">{app.repo.projectId}</span>
              </div>
            </dd>
            <dt className="lc-muted">主线分支名</dt>
            <dd><Code>{app.mainBranch}</Code></dd>
            <dt className="lc-muted">Owner</dt>
            <dd>{app.owners.map(userName).join('、')}</dd>
            <dt className="lc-muted">PE</dt>
            <dd>{app.pes.length ? app.pes.map(userName).join('、') : <span className="lc-muted">未设置</span>}</dd>
          </dl>
          <DeployInfo appKey={app.key} value={app.deployInfo} editable={perm.editDeploy} />
        </section>
        <section className="col-span-7 max-[1100px]:col-span-12" aria-labelledby="audit">
          <SectionHead id="audit">变更历史</SectionHead>
          <ol className="divide-y divide-[var(--lc-rule)]">
            {s.audit
              .filter((e) => e.appKey === app.key)
              .map((e, i) => (
                <li key={i} className="grid grid-cols-[150px_64px_minmax(0,1fr)] gap-x-4 py-2.5 max-[1100px]:grid-cols-[136px_56px_minmax(0,1fr)]">
                  <span className="lc-num text-[var(--lc-muted)]">{e.at}</span>
                  <span className="text-[var(--lc-ink-2)]">{userName(e.actor)}</span>
                  <span>{e.text}</span>
                </li>
              ))}
          </ol>
        </section>
      </div>

      {forkFrom !== null && <ForkDialog key={forkFrom} app={app} sourceVersionId={forkFrom} onClose={() => setForkFrom(null)} onCreated={(id) => select(id)} />}
      {registerOn && <VersionDialog line={registerOn} onClose={() => setRegisterOn(null)} />}
      {statusTo && <StatusDialog line={statusTo.line} to={statusTo.to} onClose={() => setStatusTo(null)} />}
      {editing && <EditAppDialog app={app} onClose={() => setEditing(false)} />}
      {deleting && <DeleteAppDialog app={app} onClose={() => setDeleting(false)} />}
    </Shell>
  );
}

const STAGES: LineStatus[] = ['活跃', '冻结', '终止'];
const MEANING: Record<LineStatus, string> = { 活跃: '可以正常发版', 冻结: '只允许紧急修复', 终止: '不再发版，只读' };

function LinePanel({ line, mainBranch, manage, selectedVer, onSelect, onFork, onRegister, onStatus }: { line: Line; mainBranch: string; manage: boolean; selectedVer: string | null; onSelect: (lineId: string, verId?: string) => void; onFork: (verId: string) => void; onRegister: () => void; onStatus: (to: LineStatus) => void }) {
  const s = useStore();
  const vers = lineVersions(s, line.id).slice().reverse();
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const srcLine = src ? lineById(src.lineId) : undefined;
  const retired = line.status === '终止';
  const at = STAGES.indexOf(line.status);
  const exp = isExpiring(line);
  const start = lineStart(s, line);

  return (
    <section className="mt-12" aria-labelledby="line-head">
      <SectionHead
        id="line-head"
        actions={
          manage &&
          (retired ? <span className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">已终止，只读</span> : <Button onClick={onRegister}>登记版本</Button>)
        }
      >
        {line.isMain ? '主线' : line.name}
        <span className="ml-3 text-[length:var(--lc-text-base)] font-normal text-[var(--lc-muted)]">
          <Code>{line.isMain ? mainBranch : line.branch}</Code>
        </span>
      </SectionHead>

      <div className="grid grid-cols-12 gap-x-10 gap-y-8">
        <div className="col-span-5 max-[1100px]:col-span-12">
          {!line.isMain && (
            <div className="mb-6">
              <ol className="grid grid-cols-3 border border-[var(--lc-ink)]" aria-label="状态，只能向前">
                {STAGES.map((st, i) => (
                  <li
                    key={st}
                    aria-current={i === at ? 'step' : undefined}
                    className={`flex h-11 flex-col justify-center px-3 ${i > 0 ? 'border-l border-[var(--lc-ink)]' : ''} ${i === at ? 'bg-[var(--lc-ink)] text-[var(--lc-paper)]' : i < at ? 'text-[var(--lc-muted)] line-through decoration-[var(--lc-rule-2)]' : 'text-[var(--lc-ink-2)]'}`}
                  >
                    <span className="text-[length:var(--lc-text-base)] leading-tight font-bold no-underline">{st}</span>
                    <span className={`text-[length:var(--lc-text-xs)] leading-tight ${i === at ? 'text-[var(--lc-paper-3)]' : ''}`}>{MEANING[st]}</span>
                  </li>
                ))}
              </ol>
              {manage && !retired && (
                <div className="mt-3 flex items-center gap-2">
                  {line.status === '活跃' && <Button onClick={() => onStatus('冻结')}>改为冻结</Button>}
                  <Button kind="danger" onClick={() => onStatus('终止')}>改为终止</Button>
                  <span className="ml-1 text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">状态只能向前，不能撤回</span>
                </div>
              )}
            </div>
          )}

          <dl className="grid grid-cols-[96px_minmax(0,1fr)] gap-x-4 gap-y-3">
            {line.isMain ? (
              <>
                <dt className="lc-muted">生命周期</dt>
                <dd>主线持续迭代，没有计划终止日期。第一个版本在 <span className="lc-num">{start}</span>。</dd>
              </>
            ) : (
              <>
                <dt className="lc-muted">源版本</dt>
                <dd>
                  {src && srcLine ? (
                    <button type="button" className="lc-link text-left" onClick={() => onSelect(srcLine.id, src.id)}>
                      <Code className="font-semibold">{src.number}</Code>，位于{srcLine.isMain ? '主线' : `「${srcLine.name}」`}
                    </button>
                  ) : (
                    '—'
                  )}
                  <div className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                    <span className="lc-num">{start}</span> 分叉
                  </div>
                </dd>
                <dt className="lc-muted">计划终止日期</dt>
                <dd>
                  {line.plannedEnd ? <span className="lc-num">{line.plannedEnd}</span> : <span className="lc-muted">未定</span>}
                  {line.plannedEnd && !retired && <span className={`ml-2 ${exp ? 'lc-signal-text' : 'text-[var(--lc-ink-2)]'}`}>{dueText(line.plannedEnd)}</span>}
                </dd>
                <dt className="lc-muted">{retired ? '存续' : '自源版本起'}</dt>
                <dd>{span(start, lineSolidEnd(s, line))}</dd>
                <dt className="lc-muted">分叉原因</dt>
                <dd className="max-w-[60ch]">{line.reason}</dd>
                <dt className="lc-muted">备注</dt>
                <dd className={line.note ? '' : 'lc-muted'}>{line.note || '无'}</dd>
              </>
            )}
          </dl>
        </div>

        <div className="col-span-7 max-[1100px]:col-span-12">
          <div className="mb-2 flex items-baseline justify-between">
            <h3 className="text-[length:var(--lc-text-base)] font-bold">
              版本 <span className="lc-num font-normal text-[var(--lc-muted)]">{vers.length}</span>
            </h3>
            <span className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">最新在上</span>
          </div>
          {vers.length === 0 ? (
            <p className="border-y border-[var(--lc-rule)] py-4 text-[var(--lc-ink-2)]">
              分叉后还没有登记版本。{manage && !retired && <button type="button" className="lc-link" onClick={onRegister}>登记第一个版本</button>}
            </p>
          ) : (
            <ul className="border-t border-[var(--lc-ink)]">
              {vers.map((v) => {
                const kids = childLines(s, v.id);
                const on = selectedVer === v.id;
                return (
                  <li key={v.id} className={`grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-4 border-b border-[var(--lc-rule)] px-2 py-3 ${on ? 'bg-[var(--lc-paper-3)]' : ''}`}>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-baseline gap-x-4">
                        <button type="button" onClick={() => onSelect(line.id, v.id)} className="lc-link !no-underline">
                          <Code className="text-[length:var(--lc-text-base)] font-bold">{v.number}</Code>
                        </button>
                        <span className="lc-num text-[var(--lc-ink-2)]">{v.releasedAt}</span>
                        <span className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                          {v.ref.kind} <Code className="text-[var(--lc-ink-2)]">{v.ref.value}</Code>
                        </span>
                      </div>
                      {v.note && <div className="mt-1 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">{v.note}</div>}
                      {kids.length > 0 && (
                        <div className="mt-1 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
                          分叉出{' '}
                          {kids.map((k, i) => (
                            <span key={k.id}>
                              {i > 0 && '、'}
                              <button type="button" className="lc-link" onClick={() => onSelect(k.id)}>
                                {k.name}
                              </button>
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                    {manage && (
                      <Button kind="ghost" className="!h-8" onClick={() => onFork(v.id)} aria-label={`从 ${v.number} 分叉维护线`}>
                        从此版本分叉
                      </Button>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>
    </section>
  );
}

function DeployInfo({ appKey, value, editable }: { appKey: string; value: string; editable: boolean }) {
  const [draft, setDraft] = useState<string | null>(null);
  return (
    <div className="mt-10">
      <SectionHead actions={editable && draft === null && <Button className="!h-8" onClick={() => setDraft(value)}>编辑部署相关信息</Button>}>部署相关信息</SectionHead>
      {draft === null ? (
        value ? <p className="whitespace-pre-line">{value}</p> : <p className="text-[var(--lc-ink-2)]">还没有填写。{editable ? '' : 'Owner 和 PE 可以编辑。'}</p>
      ) : (
        <form onSubmit={(e) => (e.preventDefault(), actions.setDeployInfo(appKey, draft), setDraft(null))}>
          <Textarea aria-label="部署相关信息" rows={5} value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus />
          <div className="mt-3 flex gap-2">
            <Button kind="primary" type="submit">保存</Button>
            <Button onClick={() => setDraft(null)}>取消</Button>
          </div>
        </form>
      )}
    </div>
  );
}
