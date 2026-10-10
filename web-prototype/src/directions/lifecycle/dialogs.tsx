import { useState } from 'react';
import { actions, appVersions, lineageOrder, lineById, lineVersions, TODAY, useStore, validateApp, verifyRepo, versionById, type AppInput, type Application, type Line, type LineStatus } from '../../data';
import { navigate } from '../../router';
import { daysBetween } from './time';
import { Button, Code, Dialog, DialogActions, Fact, Field, Input, Select, Textarea, UserPicker } from './ui';

const lineLabel = (l?: Line) => (l ? (l.isMain ? '主线' : l.name) : '');

// ---------- 分叉维护线 ----------

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
  const taken = b && (b === app.mainBranch || s.lines.some((l) => l.appKey === app.key && l.branch === b));
  const errors = {
    src: !src ? '选择源版本' : undefined,
    name: !name.trim() ? '填写维护线名称' : undefined,
    branch: !b ? '填写 Git 分支名' : taken ? `本应用已有线使用 ${b}，换一个分支名` : undefined,
    reason: !reason.trim() ? '分叉原因必填：写清楚为什么不能在主线上解决' : undefined,
    plannedEnd: plannedEnd && plannedEnd < TODAY ? '计划终止日期不能早于今天' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);
  const show = (k: keyof typeof errors) => (tried || (k === 'branch' && taken) ? errors[k] : undefined);

  const srcV = src ? versionById(src) : undefined;
  const submit = () => {
    setTried(true);
    if (!ok) return;
    const id = actions.fork({ appKey: app.key, name: name.trim(), branch: b, sourceVersionId: src, reason: reason.trim(), plannedEnd: plannedEnd || undefined, note: note.trim() || undefined });
    onCreated(id);
    onClose();
  };

  return (
    <Dialog onClose={onClose} title="分叉维护线" description="新维护线从源版本的发布时间开始画在时间轴上，状态为活跃。Owner 和 PE 沿用应用的设置。" width={600}>
      <form onSubmit={(e) => (e.preventDefault(), submit())} noValidate>
        <Field label="源版本" error={show('src')} help={srcV ? `位于${lineLabel(lineById(srcV.lineId))}，${srcV.releasedAt} 发布，距今 ${daysBetween(srcV.releasedAt, TODAY)} 天` : '任何线上的版本都可以作为源版本'}>
          <Select value={src} onChange={(e) => setSrc(e.target.value)}>
            <option value="">选择版本</option>
            {lineageOrder(s, app.key).map(({ line }) => {
              const vs = lineVersions(s, line.id);
              if (!vs.length) return null;
              return (
                <optgroup key={line.id} label={`${lineLabel(line)}${line.status === '终止' ? '（终止）' : ''}`}>
                  {vs.slice().reverse().map((v) => (
                    <option key={v.id} value={v.id}>
                      {v.number}　{v.releasedAt}
                    </option>
                  ))}
                </optgroup>
              );
            })}
          </Select>
        </Field>
        <div className="mt-2 grid grid-cols-2 gap-4">
          <Field label="名称" error={show('name')}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ACME 定制" autoFocus />
          </Field>
          <Field label="Git 分支名" error={show('branch')} help="只登记，不会在仓库里创建分支">
            <Input value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="cust/acme" className="lc-code" />
          </Field>
        </div>
        <Field className="mt-2" label="分叉原因" error={show('reason')} help="会显示在维护线详情里，其他 Owner 靠它判断这条线还要不要维护">
          <Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="客户合同要求定制对账单格式，主线不接受这部分改动" />
        </Field>
        <div className="mt-2 grid grid-cols-2 gap-4">
          <Field label="计划终止日期" error={show('plannedEnd')} help={plannedEnd && !errors.plannedEnd ? `从今天起 ${daysBetween(TODAY, plannedEnd)} 天；进入最后 30 天时首页标红` : '可选。分叉后不能在这里修改；不填则图上显示「未定计划终止日期」'}>
            <Input type="date" min={TODAY} value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
          </Field>
          <Field label="备注" help="可选">
            <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
        </div>
        <DialogActions>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit">分叉</Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

// ---------- 登记版本 ----------

export function VersionDialog({ line, onClose }: { line: Line; onClose: () => void }) {
  const s = useStore();
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
  const errors = {
    number: !n ? '填写版本号' : clash ? `${n} 已登记在${clashLine?.id === line.id ? '这条线' : lineLabel(clashLine)}（${clash.releasedAt}），同一应用内版本号不能重复` : undefined,
    ref: !ref.trim() ? `填写对应的 ${kind}` : undefined,
    releasedAt: !releasedAt ? '填写发布时间' : undefined,
    note: frozen && !note.trim() ? '冻结的维护线只接受紧急修复，写明修了什么' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);
  const show = (k: keyof typeof errors) => (tried || (k === 'number' && clash) ? errors[k] : undefined);
  const last = lineVersions(s, line.id).at(-1);

  const submit = () => {
    setTried(true);
    if (!ok) return;
    actions.registerVersion({ lineId: line.id, number: n, ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onClose();
  };

  return (
    <Dialog
      onClose={onClose}
      title={`在${line.isMain ? '主线' : `「${line.name}」`}登记版本`}
      description={
        <>
          分支 <Code>{line.branch}</Code>
          {last ? <>，上一个版本 <Code>{last.number}</Code>（{last.releasedAt}）</> : '，这是第一个版本'}。人工登记，不会去仓库核对 tag 或 commit。
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())} noValidate>
        <Field label="版本号" error={show('number')} help="格式不限，同一应用内唯一">
          <Input value={number} onChange={(e) => setNumber(e.target.value)} className="lc-code" autoFocus />
        </Field>
        <Field className="mt-2" label="对应的 tag 或 commit" error={show('ref')}>
          <div className="flex gap-2">
            <div className="lc-seg shrink-0" role="group" aria-label="引用类型">
              {(['tag', 'commit'] as const).map((k) => (
                <button key={k} type="button" aria-pressed={kind === k} onClick={() => setKind(k)} className="!h-[34px]">
                  {k}
                </button>
              ))}
            </div>
            <Input aria-label={kind} value={ref} onChange={(e) => setRef(e.target.value)} className="lc-code" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} />
          </div>
        </Field>
        <Field className="mt-2" label="发布时间" error={show('releasedAt')} help="决定版本在时间轴上的位置">
          <Input type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} />
        </Field>
        <Field className="mt-2" label={frozen ? '说明（冻结状态下必填）' : '说明'} error={show('note')} help={frozen ? '这条线已冻结，只接受紧急修复' : '可选'}>
          <Textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
        <DialogActions>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit">登记</Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

// ---------- 状态：活跃 → 冻结 → 终止，只能向前 ----------

export function StatusDialog({ line, to, onClose }: { line: Line; to: LineStatus; onClose: () => void }) {
  const text =
    to === '冻结'
      ? '冻结后只允许登记紧急修复版本，每个版本都要写说明。冻结不能撤回，之后只能改为终止。'
      : '终止后这条线只读：不能再登记版本，也不能删除，记录会一直保留在图上。终止不能撤回。';
  return (
    <Dialog onClose={onClose} title={`将「${line.name}」改为${to}？`} description={text}>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind={to === '终止' ? 'danger' : 'default'} onClick={() => (actions.setLineStatus(line.id, to), onClose())} autoFocus>
          改为{to}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// ---------- 应用：创建 / 编辑 / 删除 ----------

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [f, setF] = useState<AppInput>({ key: '', name: '', description: '', platformId: s.platforms[0]?.id ?? '', repoPath: '', projectId: '', mainBranch: 'main', owners: [], pes: [] });
  const [repoMsg, setRepoMsg] = useState<string | null>(null);
  const [tried, setTried] = useState(false);
  const set = (patch: Partial<AppInput>) => setF((x) => ({ ...x, ...patch }));
  const errors = validateApp(f, 'create');
  const err = (k: keyof AppInput) => (tried ? errors[k] : undefined);

  const verify = () => {
    const r = verifyRepo(f.platformId, f.repoPath);
    if (r.ok) set({ projectId: r.projectId }), setRepoMsg(null);
    else set({ projectId: '' }), setRepoMsg(r.message);
  };
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.createApp(f);
    onClose();
    navigate(`/apps/${f.key}`);
  };
  const platform = s.platforms.find((p) => p.id === f.platformId);

  return (
    <Dialog onClose={onClose} title="创建应用" description="创建后自动带一条主线。应用标识和仓库关联之后不能修改。" width={680}>
      <form onSubmit={(e) => (e.preventDefault(), submit())} noValidate>
        <div className="grid grid-cols-2 gap-x-4">
          <Field label="应用标识" error={err('key')} help="小写字母开头，可用 a–z、0–9 和 -；创建后不可修改">
            <Input value={f.key} onChange={(e) => set({ key: e.target.value.trim() })} placeholder="billing-core" className="lc-code" autoFocus />
          </Field>
          <Field label="显示名" error={err('name')}>
            <Input value={f.name} onChange={(e) => set({ name: e.target.value })} placeholder="计费核心" />
          </Field>
        </div>
        <Field className="mt-2" label="描述" help="可选，一句话说明应用做什么">
          <Input value={f.description} onChange={(e) => set({ description: e.target.value })} />
        </Field>

        <fieldset className="mt-2 border-t border-[var(--lc-rule)] pt-3">
          <legend className="lc-h2 pr-2 !text-[length:var(--lc-text-base)]">仓库关联</legend>
          <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] items-start gap-x-3">
            <Field label="代码平台" error={err('platformId')}>
              <Select value={f.platformId} onChange={(e) => (set({ platformId: e.target.value, projectId: '' }), setRepoMsg(null))}>
                {s.platforms.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.type} · {new URL(p.url).host}
                  </option>
                ))}
              </Select>
            </Field>
            <Field
              label="仓库路径"
              error={repoMsg ?? err('repoPath')}
              help={f.projectId ? <span className="font-semibold text-[var(--lc-ink)]">已校验：{platform?.type} 项目 ID <span className="lc-num">{f.projectId}</span></span> : '填 group/repo，然后校验'}
            >
              <Input value={f.repoPath} onChange={(e) => (set({ repoPath: e.target.value, projectId: '' }), setRepoMsg(null))} placeholder="trade/billing-core" className="lc-code" />
            </Field>
            <div className="pt-[22px]">
              <Button onClick={verify} disabled={!f.repoPath.trim()}>
                校验
              </Button>
            </div>
          </div>
          <Field label="主线分支名" error={err('mainBranch')} className="w-1/2 pr-2">
            <Input value={f.mainBranch} onChange={(e) => set({ mainBranch: e.target.value })} className="lc-code" />
          </Field>
        </fieldset>

        <fieldset className="mt-2 border-t border-[var(--lc-rule)] pt-3">
          <legend className="lc-h2 pr-2 !text-[length:var(--lc-text-base)]">Owner 和 PE</legend>
          <div className="grid grid-cols-2 gap-x-4">
            <UserPicker label="Owner" value={f.owners} onChange={(owners) => set({ owners })} users={s.users} error={err('owners')} help="至少一个，只能选正常状态的用户" />
            <UserPicker label="PE" value={f.pes} onChange={(pes) => set({ pes })} users={s.users} help="可选" />
          </div>
        </fieldset>

        <DialogActions left={tried && Object.keys(errors).length > 0 && <span className="text-[length:var(--lc-text-sm)] font-semibold text-[var(--lc-error)]">还有 {Object.keys(errors).length} 处需要修改</span>}>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit">创建应用</Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

export function EditAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const s = useStore();
  const [f, setF] = useState({ name: app.name, description: app.description, mainBranch: app.mainBranch, owners: app.owners, pes: app.pes });
  const set = (patch: Partial<typeof f>) => setF((x) => ({ ...x, ...patch }));
  const input: AppInput = { ...f, key: app.key, platformId: app.repo.platformId, repoPath: app.repo.path, projectId: app.repo.projectId };
  const errors = validateApp(input, 'edit');
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const submit = () => {
    if (Object.keys(errors).length) return;
    actions.updateApp(app.key, f);
    onClose();
  };
  return (
    <Dialog onClose={onClose} title="编辑应用" description="Owner 和 PE 的增减会逐条写进变更历史。" width={640}>
      <form onSubmit={(e) => (e.preventDefault(), submit())} noValidate>
        <div className="grid grid-cols-2 gap-x-4">
          <Fact label="应用标识" help="创建后不可修改">
            <Code>{app.key}</Code>
          </Fact>
          <Fact label="仓库关联" help="创建后不可修改">
            <span className="truncate">
              <Code>{app.repo.path}</Code>
              <span className="ml-2 text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">{platform?.type} · 项目 ID {app.repo.projectId}</span>
            </span>
          </Fact>
          <Field label="显示名" error={errors.name}>
            <Input value={f.name} onChange={(e) => set({ name: e.target.value })} />
          </Field>
          <Field label="主线分支名" error={errors.mainBranch}>
            <Input value={f.mainBranch} onChange={(e) => set({ mainBranch: e.target.value })} className="lc-code" />
          </Field>
        </div>
        <Field className="mt-2" label="描述">
          <Textarea rows={2} value={f.description} onChange={(e) => set({ description: e.target.value })} />
        </Field>
        <div className="grid grid-cols-2 gap-x-4">
          <UserPicker label="Owner" value={f.owners} onChange={(owners) => set({ owners })} users={s.users} error={errors.owners} help="至少保留一个 Owner" />
          <UserPicker label="PE" value={f.pes} onChange={(pes) => set({ pes })} users={s.users} help="PE 只能编辑部署相关信息" />
        </div>
        <DialogActions>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit" disabled={Object.keys(errors).length > 0}>
            保存
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

export function DeleteAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const s = useStore();
  const [typed, setTyped] = useState('');
  const lines = s.lines.filter((l) => l.appKey === app.key);
  const vers = appVersions(s, app.key);
  const match = typed.trim() === app.key;
  return (
    <Dialog onClose={onClose} title={`删除应用「${app.name}」？`} description={`会同时删除 ${lines.length} 条线和 ${vers.length} 个版本的登记记录，不能恢复。仓库本身不受影响。`}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (!match) return;
          actions.deleteApp(app.key);
          onClose();
          navigate('/apps');
        }}
      >
        <Field label={<>输入应用标识 <Code>{app.key}</Code> 确认</>} help="和应用标识完全一致才能删除">
          <Input value={typed} onChange={(e) => setTyped(e.target.value)} className="lc-code" autoFocus autoComplete="off" />
        </Field>
        <DialogActions>
          <Button onClick={onClose}>取消</Button>
          <Button kind="danger" type="submit" disabled={!match}>
            删除应用
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
