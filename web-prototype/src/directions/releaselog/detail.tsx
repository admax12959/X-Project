import { useState, type ReactNode } from 'react';
import { actions, appVersions, can, childLines, lineageOrder, lineById, lineVersions, TODAY, useStore, userName, versionById, type Application, type Line } from '../../data';
import { navigate, setParam, useLocation } from '../../router';
import { DeleteAppDialog, EditAppDialog } from './appforms';
import { VersionGraph } from './graph';
import { Shell } from './Shell';
import { Button, Dialog, expiryWords, Field, Input, LinkButton, Mono, Segmented, Select, STATUS_MEANING, StatusWord, Textarea } from './ui';

export function AppDetail({ appKey }: { appKey: string }) {
  const s = useStore();
  const { params } = useLocation();
  const [forkFrom, setForkFrom] = useState<string | null>(null);
  const [registerOn, setRegisterOn] = useState<string | null>(null);
  const [statusTo, setStatusTo] = useState<{ line: Line; to: Line['status'] } | null>(null);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const app = s.apps.find((a) => a.key === appKey);
  if (!app)
    return (
      <Shell crumbs={[{ label: '应用', to: '/apps' }, { label: appKey }]}>
        <div className="px-8 pt-10">
          <h1 className="rl-t-xl">没有应用标识为 <Mono>{appKey}</Mono> 的应用</h1>
          <p className="mt-2 rl-ink-2">可能已被管理员删除。<LinkButton onClick={() => navigate('/apps')}>回到应用列表</LinkButton></p>
        </div>
      </Shell>
    );

  const perm = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const verParam = params.get('ver');
  const ver = verParam && versionById(verParam) ? verParam : null;
  const lineParam = params.get('line');
  const line = !ver ? (lineParam && lineById(lineParam)?.appKey === appKey ? lineParam : `${appKey}:main`) : null;
  const lines = lineageOrder(s, appKey);
  const versions = appVersions(s, appKey);

  const selectLine = (id: string) => (setParam('ver', null), setParam('line', id));
  const selectVersion = (id: string) => (setParam('line', null), setParam('ver', id));

  return (
    <Shell crumbs={[{ label: '应用', to: '/apps' }, { label: app.name }]}>
      {/* header strip: who/what this 应用 is, compact */}
      <header className="px-8 pt-7 pb-5">
        <div className="flex flex-wrap items-start justify-between gap-x-8 gap-y-3">
          <div className="min-w-0">
            <h1 className="rl-t-xl">{app.name}</h1>
            {app.description && <p className="mt-1 rl-ink-2">{app.description}</p>}
          </div>
          <div className="flex shrink-0 gap-2">
            {perm.manageLines && <Button onClick={() => setForkFrom('')}>分叉维护线</Button>}
            {perm.editApp && <Button onClick={() => setEditing(true)}>编辑应用</Button>}
            {perm.admin && <Button onClick={() => setDeleting(true)}>删除应用</Button>}
          </div>
        </div>
        <dl className="mt-4 flex flex-wrap gap-x-8 gap-y-2">
          <Fact k="应用标识"><Mono className="rl-ink">{app.key}</Mono></Fact>
          <Fact k="仓库关联">
            <Mono className="rl-ink">{app.repo.path}</Mono>
            <span className="ml-2 rl-ink-3">{platform ? `${platform.type} ${new URL(platform.url).host}` : '代码平台已移除'} · 项目 ID {app.repo.projectId}</span>
          </Fact>
          <Fact k="主线分支"><Mono className="rl-ink">{app.mainBranch}</Mono></Fact>
        </dl>
      </header>

      <div className="grid grid-cols-1 min-[1280px]:grid-cols-[minmax(0,1fr)_300px] min-[1536px]:grid-cols-[minmax(0,1fr)_340px]" style={{ borderTop: '1px solid var(--rl-rule)' }}>
        <section aria-labelledby="ledger-h" className="min-w-0 px-6 pt-6 max-[1279px]:overflow-x-auto min-[1536px]:px-8">
          <div className="mb-3 flex flex-wrap items-baseline gap-x-6 gap-y-2">
            <h2 id="ledger-h" className="rl-t-lg">版本流水</h2>
            <span className="rl-ink-2">{versions.length} 个版本 · 主线和 {lines.length - 1} 条维护线 · 新的在上</span>
            <Legend />
          </div>
          <div>
            <VersionGraph appKey={app.key} mainBranch={app.mainBranch} sel={{ line, ver }}
              onLine={selectLine}
              onVersion={selectVersion}
              empty={
                <p className="rl-prose rl-t-md rl-ink-2">
                  还没有登记版本。{perm.manageLines ? '在右侧主线面板「登记版本」，第一行就会出现在这里。' : 'Owner 登记版本后会出现在这里。'}
                </p>
              }
            />
          </div>
        </section>

        <aside className="min-[1280px]:sticky min-[1280px]:top-14 min-[1280px]:h-[calc(100vh-56px)] min-[1280px]:overflow-y-auto" style={{ borderLeft: '1px solid var(--rl-rule)', background: 'var(--rl-paper-2)' }}>
          <div className="px-6 pt-6 pb-8">
            {ver ? (
              <VersionPanel id={ver} canManage={perm.manageLines} onFork={setForkFrom} onLine={selectLine} />
            ) : line ? (
              <LinePanel id={line} app={app} canManage={perm.manageLines} onRegister={setRegisterOn} onStatus={(l, to) => setStatusTo({ line: l, to })} onLine={selectLine} onVersion={selectVersion} />
            ) : null}
          </div>
          <AppBasics app={app} canEditDeploy={perm.editDeploy} />
        </aside>
      </div>

      {forkFrom !== null && <ForkDialog key={forkFrom} app={app} sourceVersionId={forkFrom} onClose={() => setForkFrom(null)} onCreated={selectLine} />}
      {registerOn && <VersionDialog key={registerOn} lineId={registerOn} onClose={() => setRegisterOn(null)} />}
      {statusTo && <StatusDialog line={statusTo.line} to={statusTo.to} onClose={() => setStatusTo(null)} />}
      {editing && <EditAppDialog app={app} onClose={() => setEditing(false)} />}
      {deleting && <DeleteAppDialog app={app} onClose={() => setDeleting(false)} />}
    </Shell>
  );
}

function Fact({ k, children }: { k: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 items-baseline gap-2">
      <dt className="shrink-0 rl-ink-3">{k}</dt>
      <dd className="min-w-0">{children}</dd>
    </div>
  );
}

function Legend() {
  const sample = (kind: 'live' | 'frozen' | 'retired') => (
    <svg width="34" height="14" aria-hidden className="shrink-0">
      {kind === 'live' && <path className="rl-g-rail" d="M0,7 H34" />}
      {kind === 'frozen' && (
        <>
          <path className="rl-g-rail rl-g-rail--frozen" d="M0,5 H34" />
          <path className="rl-g-rail rl-g-rail--frozen" d="M0,9 H34" />
        </>
      )}
      {kind === 'retired' && (
        <>
          <path className="rl-g-rail rl-g-rail--retired" d="M0,7 H22" />
          <path className="rl-g-cap" d="M22,1 V13" />
          <path className="rl-g-ghost" d="M26,7 H34" />
        </>
      )}
      <circle className={kind === 'live' ? 'rl-g-node--live' : kind === 'frozen' ? 'rl-g-node--frozen' : 'rl-g-node--retired'} cx="12" cy="7" r={kind === 'retired' ? 4 : 5} />
    </svg>
  );
  return (
    <span className="ml-auto flex flex-wrap items-center gap-x-5 gap-y-1 rl-ink-2">
      <span className="flex items-center gap-2">{sample('live')}活跃</span>
      <span className="flex items-center gap-2">{sample('frozen')}冻结</span>
      <span className="flex items-center gap-2">{sample('retired')}终止</span>
    </span>
  );
}

function Dl({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className="grid grid-cols-[88px_minmax(0,1fr)] gap-x-3 gap-y-3">
      {rows.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="rl-ink-3">{k}</dt>
          <dd className="min-w-0">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

function LinePanel({ id, app, canManage, onRegister, onStatus, onLine, onVersion }: { id: string; app: Application; canManage: boolean; onRegister: (id: string) => void; onStatus: (l: Line, to: Line['status']) => void; onLine: (id: string) => void; onVersion: (id: string) => void }) {
  const s = useStore();
  const line = s.lines.find((l) => l.id === id);
  if (!line) return null;
  const vers = lineVersions(s, line.id);
  const last = vers.at(-1);
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const srcLine = src ? lineById(src.lineId) : undefined;
  const due = expiryWords(line);
  const retired = line.status === '终止';

  return (
    <div>
      <div className="rl-label">{line.isMain ? '主线' : '维护线'}</div>
      <div className="mt-1 flex items-center gap-3">
        <h2 className="rl-t-lg">{line.isMain ? '主线' : line.name}</h2>
        <StatusWord line={line} stamp />
      </div>
      <p className="mt-1 rl-ink-2">{STATUS_MEANING[line.status]}</p>

      {canManage && !retired && (
        <div className="mt-4">
          <Button kind="primary" onClick={() => onRegister(line.id)}>登记版本</Button>
        </div>
      )}

      <div className="mt-6">
        <Dl
          rows={[
            ['Git 分支', <Mono className="rl-ink">{line.isMain ? app.mainBranch : line.branch}</Mono>],
            ...(!line.isMain
              ? ([
                  ['源版本', src ? <span><LinkButton onClick={() => onVersion(src.id)}><Mono>{src.number}</Mono></LinkButton><span className="rl-ink-2"> · 位于 </span><LinkButton onClick={() => srcLine && onLine(srcLine.id)}>{srcLine?.isMain ? '主线' : srcLine?.name}</LinkButton></span> : '—'],
                  ['分叉原因', <span className="leading-relaxed">{line.reason}</span>],
                  ['计划终止', line.plannedEnd ? <span>{line.plannedEnd}{due && <span className="ml-2 font-semibold">{due}</span>}</span> : <span className="rl-ink-3">未设定</span>],
                  ['备注', line.note ? <span className="rl-ink-2">{line.note}</span> : <span className="rl-ink-3">—</span>],
                ] as [string, ReactNode][])
              : []),
            ['版本', last ? <span>{vers.length} 个，最新 <LinkButton onClick={() => onVersion(last.id)}><Mono>{last.number}</Mono></LinkButton> <span className="whitespace-nowrap rl-ink-3">{last.releasedAt}</span></span> : <span className="rl-ink-3">还没有登记版本</span>],
          ]}
        />
      </div>

      {!line.isMain && (
        <div className="mt-6 pt-4" style={{ borderTop: '1px solid var(--rl-rule)' }}>
          <div className="rl-label mb-2">状态</div>
          <ol className="flex items-center gap-2" aria-label="状态只能向前推进">
            {(['活跃', '冻结', '终止'] as const).map((st, i) => (
              <li key={st} className="flex items-center gap-2">
                {i > 0 && <span className="rl-ink-3" aria-hidden>→</span>}
                <span className={line.status === st ? 'font-semibold rl-ink' : 'rl-ink-3'}>{st}</span>
              </li>
            ))}
          </ol>
          {retired ? (
            <p className="mt-2 rl-ink-2">已终止，记录只读，不能删除，不能再登记版本。</p>
          ) : canManage ? (
            <div className="mt-3 flex gap-2">
              {line.status === '活跃' && <Button size="sm" onClick={() => onStatus(line, '冻结')}>冻结</Button>}
              <Button size="sm" onClick={() => onStatus(line, '终止')}>终止</Button>
            </div>
          ) : (
            <p className="mt-2 rl-ink-3">只有 Owner 能改变状态。</p>
          )}
        </div>
      )}
    </div>
  );
}

function VersionPanel({ id, canManage, onFork, onLine }: { id: string; canManage: boolean; onFork: (id: string) => void; onLine: (id: string) => void }) {
  const s = useStore();
  const v = s.versions.find((x) => x.id === id);
  if (!v) return null;
  const line = lineById(v.lineId)!;
  const forks = childLines(s, v.id);
  return (
    <div>
      <div className="rl-label">版本</div>
      <h2 className="mt-1 rl-t-xl rl-mono">{v.number}</h2>
      <p className="mt-1">
        <span className="rl-ink-2">位于 </span>
        <LinkButton onClick={() => onLine(line.id)}>{line.isMain ? '主线' : line.name}</LinkButton>
        <span className="ml-2"><StatusWord line={line} /></span>
      </p>
      {canManage && (
        <div className="mt-4">
          <Button kind="primary" onClick={() => onFork(v.id)}>从此版本分叉</Button>
        </div>
      )}
      <div className="mt-6">
        <Dl
          rows={[
            ['发布时间', v.releasedAt],
            [v.ref.kind, <Mono className="rl-ink">{v.ref.value}</Mono>],
            ['说明', v.note ? <span className="leading-relaxed">{v.note}</span> : <span className="rl-ink-3">没有说明</span>],
            ['分叉出', forks.length ? <span className="flex flex-col items-start gap-1">{forks.map((f) => <LinkButton key={f.id} onClick={() => onLine(f.id)}>{f.name}</LinkButton>)}</span> : <span className="rl-ink-3">没有维护线从这里分叉</span>],
          ]}
        />
      </div>
    </div>
  );
}

function AppBasics({ app, canEditDeploy }: { app: Application; canEditDeploy: boolean }) {
  const s = useStore();
  const [draft, setDraft] = useState<string | null>(null);
  const [more, setMore] = useState(false);
  const log = s.audit.filter((e) => e.appKey === app.key);
  const shown = more ? log : log.slice(0, 6);
  return (
    <div className="px-6 pb-10">
      <section className="pt-5" style={{ borderTop: '1px solid var(--rl-ink)' }}>
        <h2 className="rl-label mb-3">Owner / PE</h2>
        <Dl
          rows={[
            ['Owner', app.owners.map(userName).join('、')],
            ['PE', app.pes.length ? app.pes.map(userName).join('、') : <span className="rl-ink-3">未设置</span>],
          ]}
        />
      </section>

      <section className="mt-6 pt-5" style={{ borderTop: '1px solid var(--rl-rule)' }}>
        <div className="mb-2 flex items-baseline justify-between">
          <h2 className="rl-label">部署相关信息</h2>
          {canEditDeploy && draft === null && <LinkButton onClick={() => setDraft(app.deployInfo)}>编辑</LinkButton>}
        </div>
        {draft === null ? (
          app.deployInfo ? <p className="whitespace-pre-line leading-relaxed">{app.deployInfo}</p> : <p className="rl-ink-3">还没有填写。Owner 和 PE 可以编辑。</p>
        ) : (
          <Field label="部署相关信息" hint="例如集群、命名空间、发布窗口、回滚方式">
            {(f) => (
              <>
                <Textarea f={f} rows={5} value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus />
                <div className="mt-2 flex gap-2">
                  <Button kind="primary" size="sm" onClick={() => (actions.setDeployInfo(app.key, draft.trim()), setDraft(null))}>保存</Button>
                  <Button size="sm" onClick={() => setDraft(null)}>取消</Button>
                </div>
              </>
            )}
          </Field>
        )}
      </section>

      <section className="mt-6 pt-5" style={{ borderTop: '1px solid var(--rl-rule)' }}>
        <h2 className="rl-label mb-2">变更历史</h2>
        {log.length === 0 ? (
          <p className="rl-ink-3">还没有记录。</p>
        ) : (
          <ol className="rl-rows">
            {shown.map((e, i) => (
              <li key={i} className="py-2">
                <div>{e.text}</div>
                <div className="rl-t-xs rl-ink-3">{e.at} · {userName(e.actor)}</div>
              </li>
            ))}
          </ol>
        )}
        {log.length > 6 && <LinkButton className="mt-2" onClick={() => setMore(!more)}>{more ? '收起' : `显示全部 ${log.length} 条`}</LinkButton>}
      </section>
    </div>
  );
}

/* ---------------- dialogs ---------------- */

export function ForkDialog({ app, sourceVersionId, onClose, onCreated }: { app: Application; sourceVersionId: string; onClose: () => void; onCreated: (lineId: string) => void }) {
  const s = useStore();
  const [src, setSrc] = useState(sourceVersionId);
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const b = branch.trim();
  const taken = s.lines.some((l) => l.appKey === app.key && (l.branch === b || l.id === `${app.key}:${b}`)) || b === app.mainBranch;
  const errors = {
    src: !src ? '选择源版本' : undefined,
    name: !name.trim() ? '填写维护线名称' : undefined,
    branch: !b ? '填写 Git 分支名' : /\s/.test(b) ? '分支名不能含空格' : taken ? `本应用已有线使用 ${b}，换一个分支名` : undefined,
    reason: !reason.trim() ? '分叉原因必填：写清楚为什么不能在主线上解决' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);
  const srcV = src ? versionById(src) : undefined;
  const srcLine = srcV ? lineById(srcV.lineId) : undefined;

  const submit = () => {
    setTried(true);
    if (!ok) return;
    const id = actions.fork({ appKey: app.key, name: name.trim(), branch: b, sourceVersionId: src, reason: reason.trim(), plannedEnd: plannedEnd || undefined, note: note.trim() || undefined });
    onCreated(id);
    onClose();
  };
  const err = (k: keyof typeof errors) => (tried || (k === 'branch' && taken) ? errors[k] : undefined);

  return (
    <Dialog
      onClose={onClose}
      width={600}
      title="分叉维护线"
      description={srcV ? <>从 <Mono className="rl-ink">{srcV.number}</Mono>（{srcLine?.isMain ? '主线' : srcLine?.name}，{srcV.releasedAt}）分叉。新维护线状态为活跃，Owner / PE 沿用应用的设置。</> : '新维护线状态为活跃，Owner / PE 沿用应用的设置。'}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>分叉</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())} className="grid grid-cols-2 gap-x-4">
        <Field className="col-span-2" label="源版本" error={err('src')} hint="任何线上的版本都可以作为源版本，包括其他维护线">
          {(f) => (
            <Select f={f} value={src} onChange={(e) => setSrc(e.target.value)}>
              <option value="">选择版本</option>
              {lineageOrder(s, app.key).map(({ line }) => {
                const vs = lineVersions(s, line.id);
                if (!vs.length) return null;
                return (
                  <optgroup key={line.id} label={`${line.isMain ? '主线' : line.name}（${line.status}）`}>
                    {vs.slice().reverse().map((v) => (
                      <option key={v.id} value={v.id}>{v.number}　{v.releasedAt}</option>
                    ))}
                  </optgroup>
                );
              })}
            </Select>
          )}
        </Field>
        <Field label="名称" error={err('name')}>{(f) => <Input f={f} value={name} onChange={(e) => setName(e.target.value)} placeholder="ACME 定制" autoFocus />}</Field>
        <Field label="Git 分支名" error={err('branch')} hint="只登记，不会在仓库里创建分支">
          {(f) => <Input f={f} value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="cust/acme" className="rl-mono" />}
        </Field>
        <Field className="col-span-2" label="分叉原因" error={err('reason')}>
          {(f) => <Textarea f={f} rows={3} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="客户合同要求定制对账单格式，主线不接受这部分改动" />}
        </Field>
        <Field label="计划终止日期" hint="到期前 30 天出现在首页">{(f) => <Input f={f} type="date" min={TODAY} value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />}</Field>
        <Field label="备注（可选）">{(f) => <Input f={f} value={note} onChange={(e) => setNote(e.target.value)} />}</Field>
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

function VersionDialog({ lineId, onClose }: { lineId: string; onClose: () => void }) {
  const s = useStore();
  const line = lineById(lineId)!;
  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<'tag' | 'commit'>('tag');
  const [ref, setRef] = useState('');
  const [releasedAt, setReleasedAt] = useState(TODAY);
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const frozen = line.status === '冻结';
  const n = number.trim();
  const clash = n ? appVersions(s, line.appKey).find((v) => v.number === n) : undefined;
  const clashLine = clash ? lineById(clash.lineId) : undefined;
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const last = lineVersions(s, line.id).at(-1);
  const errors = {
    number: !n ? '填写版本号' : clash ? `${n} 已登记在${clashLine?.isMain ? '主线' : `「${clashLine?.name}」`}（${clash.releasedAt}），同一应用内版本号不能重复` : undefined,
    ref: !ref.trim() ? `填写 ${kind}` : undefined,
    releasedAt: !releasedAt ? '填写发布时间' : src && releasedAt < src.releasedAt ? `不能早于源版本 ${src.number} 的发布时间 ${src.releasedAt}` : undefined,
    note: frozen && !note.trim() ? '冻结的维护线只允许紧急修复，写明修了什么' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);
  const submit = () => {
    setTried(true);
    if (!ok) return;
    actions.registerVersion({ lineId: line.id, number: n, ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onClose();
  };
  const err = (k: keyof typeof errors) => (tried || (k === 'number' && clash) ? errors[k] : undefined);

  return (
    <Dialog
      onClose={onClose}
      title={`在${line.isMain ? '主线' : `「${line.name}」`}登记版本`}
      description={<>分支 <Mono className="rl-ink">{line.isMain ? s.apps.find((a) => a.key === line.appKey)?.mainBranch : line.branch}</Mono>{last ? <>，上一个版本 <Mono className="rl-ink">{last.number}</Mono>（{last.releasedAt}）</> : '，这是第一个版本'}。人工登记，不校验 tag 或 commit 是否存在。</>}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>登记</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())}>
        <Field label="版本号" error={err('number')} hint="格式不限，同一应用内唯一">
          {(f) => <Input f={f} value={number} onChange={(e) => setNumber(e.target.value)} className="rl-mono" autoFocus />}
        </Field>
        <Field label="对应的 tag 或 commit" error={err('ref')}>
          {(f) => (
            <div className="flex gap-2">
              <Segmented name="refkind" label="引用类型" value={kind} onChange={setKind} options={[{ value: 'tag', label: 'tag' }, { value: 'commit', label: 'commit' }]} />
              <Input f={f} value={ref} onChange={(e) => setRef(e.target.value)} className="rl-mono" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} />
            </div>
          )}
        </Field>
        <Field label="发布时间" error={err('releasedAt')}>{(f) => <Input f={f} type="date" min={src?.releasedAt} value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} />}</Field>
        <Field label={frozen ? '说明（冻结的维护线必填）' : '说明（可选）'} error={err('note')} hint={frozen ? '例如修复了哪个安全问题' : undefined}>
          {(f) => <Textarea f={f} rows={2} value={note} onChange={(e) => setNote(e.target.value)} aria-required={frozen} />}
        </Field>
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

function StatusDialog({ line, to, onClose }: { line: Line; to: Line['status']; onClose: () => void }) {
  const text =
    to === '冻结'
      ? '冻结后只允许登记紧急修复版本，每个版本必须写说明。冻结不能撤回，之后只能终止。'
      : '终止后这条维护线只读：不能再登记版本，不能删除，也不能恢复。已登记的版本仍可作为源版本。';
  return (
    <Dialog
      onClose={onClose}
      title={`将「${line.name}」${to === '冻结' ? '冻结' : '终止'}？`}
      description={text}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind={to === '终止' ? 'danger' : 'primary'} onClick={() => (actions.setLineStatus(line.id, to), onClose())}>
            {to === '冻结' ? '冻结' : '终止'}
          </Button>
        </>
      }
    />
  );
}
