import { useState } from 'react';
import {
  actions,
  appVersions,
  lineageOrder,
  lineById,
  lineVersions,
  soleOwnerOf,
  TODAY,
  useStore,
  userName,
  validateApp,
  validateLocalUser,
  verifyRepo,
  type AppInput,
  type Application,
  type CodePlatform,
  type IdentityType,
  type Line,
  type LineStatus,
  type LocalUserInput,
  type User,
} from '../../data';
import { navigate } from '../../router';
import { Btn, Dialog, Field, IDENTITY_LABEL, Input, LinkBtn, Mono, RailGlyph, Select, Textarea } from './ui';

// ---------------------------------------------------------------- people picker

function PeoplePicker({ label, value, onChange, error, roleWord }: { label: string; value: string[]; onChange: (v: string[]) => void; error?: string; roleWord: string }) {
  const s = useStore();
  const options = s.users.filter((u) => u.status === '正常' && !value.includes(u.username));
  return (
    <div>
      <span className="wb-label">{label}</span>
      <div className={`border ${error ? 'border-accent' : 'border-line'} rounded-[2px]`}>
        {value.length === 0 && <div className="px-2.5 py-2 text-ink-3">还没有 {roleWord}</div>}
        {value.map((u) => (
          <div key={u} className="flex items-center justify-between border-b border-line-2 px-2.5 py-1.5 last:border-b-0">
            <span>
              <span className="font-medium">{userName(u)}</span>
              <span className="ml-2 text-[length:var(--wb-text-xs)] text-ink-3">{u}</span>
            </span>
            <LinkBtn onClick={() => onChange(value.filter((x) => x !== u))} className="text-[length:var(--wb-text-sm)]">移除</LinkBtn>
          </div>
        ))}
        <div className="border-t border-line-2 bg-[var(--wb-hover-row)] p-1.5">
          <select
            aria-label={`添加 ${roleWord}`}
            value=""
            onChange={(e) => e.target.value && onChange([...value, e.target.value])}
            className="wb-input h-7 text-[length:var(--wb-text-sm)]"
            disabled={options.length === 0}
          >
            <option value="">{options.length ? `添加 ${roleWord}…` : '没有可添加的用户'}</option>
            {options.map((u) => (
              <option key={u.username} value={u.username}>{u.displayName}（{u.username}）{u.dept ? ` · ${u.dept}` : ''}</option>
            ))}
          </select>
        </div>
      </div>
      {error ? <span className="wb-error">{error}</span> : <span className="wb-hint">只能选择状态正常的用户</span>}
    </div>
  );
}

// ---------------------------------------------------------------- 创建应用

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [f, setF] = useState<AppInput>({ key: '', name: '', description: '', platformId: s.platforms[0]?.id ?? '', repoPath: '', projectId: '', mainBranch: 'main', owners: [], pes: [] });
  const [verifyMsg, setVerifyMsg] = useState<string | null>(null);
  const [tried, setTried] = useState(false);
  const set = <K extends keyof AppInput>(k: K, v: AppInput[K]) => setF((p) => ({ ...p, [k]: v }));
  // changing the repo invalidates any earlier 校验
  const setRepo = (patch: Partial<AppInput>) => (setF((p) => ({ ...p, ...patch, projectId: '' })), setVerifyMsg(null));

  const errors = validateApp(f, 'create');
  // the key error is shown live once typed, so a duplicate shows up immediately
  const keyErr = f.key ? errors.key : tried ? errors.key : undefined;
  const show = (k: keyof AppInput) => (tried ? errors[k] : undefined);

  const verify = () => {
    const r = verifyRepo(f.platformId, f.repoPath);
    if (r.ok) (set('projectId', r.projectId), setVerifyMsg(null));
    else (set('projectId', ''), setVerifyMsg(r.message));
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
    <Dialog
      onClose={onClose}
      width={680}
      title="创建应用"
      description="应用标识创建后不能修改；仓库关联只在这里建立。Owner 之后可以在应用里增减。"
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>创建应用</Btn>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-x-5 gap-y-4">
        <Field label="应用标识" error={keyErr} hint="小写字母开头，可用小写字母、数字和 -，创建后不能修改">
          <Input value={f.key} onChange={(e) => set('key', e.target.value.trim())} className="font-mono" placeholder="billing-core" invalid={!!keyErr} autoFocus />
        </Field>
        <Field label="显示名" error={show('name')}>
          <Input value={f.name} onChange={(e) => set('name', e.target.value)} placeholder="计费核心" invalid={!!show('name')} />
        </Field>
        <Field label="描述" className="col-span-2">
          <Input value={f.description} onChange={(e) => set('description', e.target.value)} placeholder="一句话说明这个应用做什么" />
        </Field>

        <div className="col-span-2 border-t border-line-2 pt-4">
          <div className="mb-3 font-semibold">仓库关联</div>
          <div className="grid grid-cols-[250px_minmax(0,1fr)_auto] items-start gap-x-3">
            <Field label="代码平台" error={show('platformId')}>
              <Select value={f.platformId} onChange={(e) => setRepo({ platformId: e.target.value })}>
                {s.platforms.map((p) => (
                  <option key={p.id} value={p.id}>{p.type} · {p.url.replace(/^https?:\/\//, '')}</option>
                ))}
              </Select>
            </Field>
            <Field label="仓库路径" error={verifyMsg ?? show('repoPath')}>
              <Input value={f.repoPath} onChange={(e) => setRepo({ repoPath: e.target.value })} className="font-mono" placeholder="group/repo" invalid={!!(verifyMsg ?? show('repoPath'))} />
            </Field>
            <div>
              <span className="wb-label invisible">校验</span>
              <Btn onClick={verify} disabled={!f.repoPath.trim()} className="h-[30px]">校验</Btn>
            </div>
          </div>
          <div className="mt-2 text-[length:var(--wb-text-sm)]">
            {f.projectId ? (
              <span>
                <span className="font-semibold">已找到仓库</span>
                <span className="ml-2 text-ink-2">{platform?.type} 项目 ID <span className="font-semibold text-ink">{f.projectId}</span>，用只读服务令牌访问</span>
              </span>
            ) : (
              <span className="text-ink-3">填写路径后点“校验”，工具会用代码平台的只读服务令牌确认仓库存在并取回项目 ID。</span>
            )}
          </div>
        </div>

        <Field label="主线分支名" error={show('mainBranch')} hint="主线所在的 Git 分支，例如 main 或 develop">
          <Input value={f.mainBranch} onChange={(e) => set('mainBranch', e.target.value)} className="font-mono" invalid={!!show('mainBranch')} />
        </Field>
        <div />
        <PeoplePicker label="Owner（至少一个）" roleWord="Owner" value={f.owners} onChange={(v) => set('owners', v)} error={show('owners')} />
        <PeoplePicker label="PE" roleWord="PE" value={f.pes} onChange={(v) => set('pes', v)} />
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 编辑应用

export function EditAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const s = useStore();
  const [f, setF] = useState({ name: app.name, description: app.description, mainBranch: app.mainBranch, owners: app.owners, pes: app.pes });
  const [tried, setTried] = useState(false);
  const input: AppInput = { ...f, key: app.key, platformId: app.repo.platformId, repoPath: app.repo.path, projectId: app.repo.projectId };
  const errors = validateApp(input, 'edit');
  const show = (k: keyof AppInput) => (tried ? errors[k] : undefined);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.updateApp(app.key, f);
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={640}
      title={`编辑 ${app.name}`}
      description="应用标识和仓库关联创建后不能修改。Owner 和 PE 的每次增减都会写进变更历史。"
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>保存</Btn>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-x-5 gap-y-4">
        <Field label="应用标识" hint="不可修改">
          <Input readOnly value={app.key} className="font-mono" />
        </Field>
        <Field label="仓库关联" hint={`${platform?.type ?? ''} 项目 ID ${app.repo.projectId}，不可修改`}>
          <Input readOnly value={app.repo.path} className="font-mono" />
        </Field>
        <Field label="显示名" error={show('name')}>
          <Input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} invalid={!!show('name')} autoFocus />
        </Field>
        <Field label="主线分支名" error={show('mainBranch')}>
          <Input value={f.mainBranch} onChange={(e) => setF({ ...f, mainBranch: e.target.value })} className="font-mono" invalid={!!show('mainBranch')} />
        </Field>
        <Field label="描述" className="col-span-2">
          <Textarea rows={2} value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} />
        </Field>
        <PeoplePicker label="Owner（至少一个）" roleWord="Owner" value={f.owners} onChange={(v) => setF({ ...f, owners: v })} error={show('owners')} />
        <PeoplePicker label="PE" roleWord="PE" value={f.pes} onChange={(v) => setF({ ...f, pes: v })} />
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 删除应用

export function DeleteAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const s = useStore();
  const [typed, setTyped] = useState('');
  const lineCount = s.lines.filter((l) => l.appKey === app.key && !l.isMain).length;
  const verCount = appVersions(s, app.key).length;
  return (
    <Dialog
      onClose={onClose}
      title={`删除 ${app.name}`}
      description={`应用、它的 ${lineCount} 条维护线和 ${verCount} 个版本记录会一起删除，不能恢复。代码平台上的仓库不受影响。`}
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn
            kind="solid-danger"
            disabled={typed !== app.key}
            onClick={() => {
              navigate('/apps');
              actions.deleteApp(app.key);
            }}
          >
            删除应用
          </Btn>
        </>
      }
    >
      <Field label={<>输入应用标识 <Mono>{app.key}</Mono> 确认</>}>
        <Input value={typed} onChange={(e) => setTyped(e.target.value)} className="font-mono" autoFocus />
      </Field>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 分叉维护线

export function ForkDialog({ appKey, mainBranch, sourceVersionId, onClose, onCreated }: { appKey: string; mainBranch: string; sourceVersionId: string; onClose: () => void; onCreated: (lineId: string) => void }) {
  const s = useStore();
  const [src, setSrc] = useState(sourceVersionId);
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const b = branch.trim();
  const lines = s.lines.filter((l) => l.appKey === appKey);
  const taken = lines.find((l) => (l.isMain ? mainBranch : l.branch) === b || l.id === `${appKey}:${b}`);
  const errors = {
    src: !src ? '选择源版本' : undefined,
    name: !name.trim() ? '填写名称' : undefined,
    branch: !b ? '填写 Git 分支名' : taken ? `已被「${taken.isMain ? '主线' : taken.name}」使用，同一应用内分支名不能重复` : undefined,
    reason: !reason.trim() ? '写明为什么这部分改动不能留在主线上' : undefined,
  };
  const show = (k: keyof typeof errors) => (k === 'branch' && taken ? errors.branch : tried ? errors[k] : undefined);
  const srcVer = s.versions.find((v) => v.id === src);
  const srcLine = srcVer ? lineById(srcVer.lineId) : undefined;

  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    const id = actions.fork({ appKey, name: name.trim(), branch: b, sourceVersionId: src, reason: reason.trim(), plannedEnd: plannedEnd || undefined, note: note.trim() || undefined });
    onCreated(id);
    onClose();
  };

  return (
    <Dialog
      onClose={onClose}
      width={600}
      title="分叉维护线"
      description="新维护线从源版本开始独立演进，有自己的版本序列，创建后状态为活跃。这里只做登记，不会在仓库里建分支。"
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>分叉</Btn>
        </>
      }
    >
      <div className="space-y-4">
        <Field label="源版本" error={show('src')} hint={srcVer && srcLine ? `${srcLine.isMain ? '主线' : `维护线「${srcLine.name}」`}上的版本，发布于 ${srcVer.releasedAt}${srcLine.status === '终止' ? '。这条线已终止，仍可作为源版本' : ''}` : '任何线上的版本都可以作为源版本'}>
          <Select value={src} onChange={(e) => setSrc(e.target.value)}>
            <option value="">选择版本</option>
            {lineageOrder(s, appKey).map(({ line }) => {
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
        </Field>
        <div className="grid grid-cols-2 gap-x-5">
          <Field label="名称" error={show('name')}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：ACME 定制" invalid={!!show('name')} autoFocus />
          </Field>
          <Field label="Git 分支名" error={show('branch')} hint="同一应用内唯一">
            <Input value={branch} onChange={(e) => setBranch(e.target.value)} className="font-mono" placeholder="cust/acme" invalid={!!show('branch')} />
          </Field>
        </div>
        <Field label="分叉原因" error={show('reason')} hint="写给半年后接手的人看：客户、合同、合规要求，以及为什么不能并入主线">
          <Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} invalid={!!show('reason')} />
        </Field>
        <div className="grid grid-cols-2 gap-x-5">
          <Field label="计划终止日期" hint="到期前 30 天会出现在应用列表顶部">
            <Input type="date" min={TODAY} value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
          </Field>
          <Field label="备注">
            <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
        </div>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 登记版本

export function RegisterVersionDialog({ line, mainBranch, onClose, onDone }: { line: Line; mainBranch: string; onClose: () => void; onDone?: (number: string) => void }) {
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
    number: !n ? '填写版本号' : clash ? `${clashLine?.isMain ? '主线' : `「${clashLine?.name}」`}已有版本 ${n}（${clash.releasedAt}），版本号在应用内不能重复` : undefined,
    ref: !ref.trim() ? `填写 ${kind}` : undefined,
    releasedAt: !releasedAt ? '选择发布时间' : undefined,
    note: frozen && !note.trim() ? '这条维护线已冻结，只允许紧急修复，写明修了什么' : undefined,
  };
  const show = (k: keyof typeof errors) => (k === 'number' && clash ? errors.number : tried ? errors[k] : undefined);
  const last = lineVersions(s, line.id).at(-1);

  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.registerVersion({ lineId: line.id, number: n, ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onDone?.(n);
    onClose();
  };

  return (
    <Dialog
      onClose={onClose}
      title={`在${line.isMain ? '主线' : `「${line.name}」`}登记版本`}
      description={
        <>
          分支 <Mono>{line.isMain ? mainBranch : line.branch}</Mono>
          {last ? `，上一个版本 ${last.number}（${last.releasedAt}）` : '，这条线还没有版本'}。人工登记，不校验 tag 或 commit 是否存在。
        </>
      }
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>登记</Btn>
        </>
      }
    >
      {frozen && <p className="mb-4 font-semibold">这条维护线已冻结：只登记紧急修复，说明必填。</p>}
      <div className="space-y-4">
        <Field label="版本号" error={show('number')} hint="格式不限，同一应用内唯一">
          <Input value={number} onChange={(e) => setNumber(e.target.value)} invalid={!!show('number')} autoFocus />
        </Field>
        <div>
          <span className="wb-label">对应的 tag 或 commit</span>
          <div className="flex gap-2">
            <div role="radiogroup" aria-label="类型" className="flex shrink-0 rounded-[2px] border border-line">
              {(['tag', 'commit'] as const).map((k) => (
                <button key={k} type="button" role="radio" aria-checked={kind === k} onClick={() => setKind(k)} className={`px-3 text-[length:var(--wb-text-sm)] ${kind === k ? 'bg-ink font-semibold text-white' : 'text-ink-2 hover:bg-[var(--wb-hover-row)]'}`}>
                  {k}
                </button>
              ))}
            </div>
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="font-mono" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} invalid={!!show('ref')} aria-label={kind} />
          </div>
          {show('ref') && <span className="wb-error">{show('ref')}</span>}
        </div>
        <Field label="发布时间" error={show('releasedAt')}>
          <Input type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} className="w-44" />
        </Field>
        <Field label={frozen ? '说明（必填）' : '说明'} error={show('note')}>
          <Textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} invalid={!!show('note')} />
        </Field>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 状态变更

const NEXT: Partial<Record<LineStatus, LineStatus>> = { 活跃: '冻结', 冻结: '终止' };
export const nextStatus = (l: Line) => (l.isMain ? undefined : NEXT[l.status]);

export function StatusDialog({ line, to, onClose }: { line: Line; to: LineStatus; onClose: () => void }) {
  const text =
    to === '冻结'
      ? '冻结后只允许紧急修复：仍可登记版本，但每个版本都必须写说明。冻结不能撤回到活跃。'
      : '终止后不能再登记版本，这条维护线只保留只读记录，也不能删除。它的版本仍可作为源版本分叉。终止不能撤回。';
  return (
    <Dialog
      onClose={onClose}
      title={`将「${line.name}」改为${to}`}
      description={text}
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind={to === '终止' ? 'solid-danger' : 'primary'} onClick={() => (actions.setLineStatus(line.id, to), onClose())}>改为{to}</Btn>
        </>
      }
    >
      <div className="flex items-center gap-3">
        {(['活跃', '冻结', '终止'] as const).map((st, i) => (
          <span key={st} className="flex items-center gap-3">
            {i > 0 && <span className="text-ink-3">→</span>}
            <span className={`flex items-center gap-1.5 ${st === line.status ? 'text-ink-3 line-through' : st === to ? 'font-bold' : 'text-ink-3'}`}>
              <RailGlyph status={st} width={20} />
              {st}
            </span>
          </span>
        ))}
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 登录身份

export function BindDialog({ user, type, by, onClose }: { user: User; type: IdentityType; by: string; onClose: () => void }) {
  const [login, setLogin] = useState(type === 'local' ? user.username : '');
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [tried, setTried] = useState(false);
  const ldap = type === 'ldap';
  const errors = {
    login: !login.trim() ? (ldap ? '填写 LDAP 用户名' : '填写登录名') : undefined,
    pw: !pw ? '填写密码' : !ldap && pw.length < 8 ? '至少 8 位' : undefined,
    pw2: !ldap && pw !== pw2 ? '两次输入不一致' : undefined,
  };
  const show = (k: keyof typeof errors) => (tried ? errors[k] : undefined);
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy: by });
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={460}
      title={`为 ${user.displayName} 绑定${IDENTITY_LABEL[type]}`}
      description={ldap ? '用要绑定的 LDAP 身份完成一次认证，证明这个身份属于该用户。绑定后显示名、邮箱、部门改为以 LDAP 为准。' : '设置一组本地登录名和密码，之后可以不经过 LDAP 登录。'}
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>{ldap ? '认证并绑定' : '绑定'}</Btn>
        </>
      }
    >
      <div className="space-y-4">
        <Field label={ldap ? 'LDAP 用户名' : '本地登录名'} error={show('login')}>
          <Input value={login} onChange={(e) => setLogin(e.target.value)} invalid={!!show('login')} autoFocus />
        </Field>
        <Field label={ldap ? 'LDAP 密码' : '密码'} error={show('pw')}>
          <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} invalid={!!show('pw')} />
        </Field>
        {!ldap && (
          <Field label="再输一次密码" error={show('pw2')}>
            <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} invalid={!!show('pw2')} />
          </Field>
        )}
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 代码平台

export function PlatformDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [type, setType] = useState<CodePlatform['type']>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [test, setTest] = useState<null | { ok: boolean; msg: string }>(null);
  const reset = () => setTest(null);
  const runTest = () => {
    const u = url.trim().replace(/\/$/, '');
    if (!/^https?:\/\/[\w.-]+(:\d+)?$/.test(u)) return setTest({ ok: false, msg: '实例地址格式应为 https://主机名' });
    if (s.platforms.some((p) => p.url === u)) return setTest({ ok: false, msg: '这个实例已经登记过' });
    if (token.length < 8) return setTest({ ok: false, msg: `${type} 返回 401：令牌无效` });
    setTest({ ok: true, msg: `连接成功：${type} 接受了令牌，权限为只读` });
  };
  return (
    <Dialog
      onClose={onClose}
      width={500}
      title="登记代码平台"
      description="应用建立仓库关联时从这里选择平台。令牌只需要只读权限，保存后只显示末四位。"
      footer={
        <>
          <Btn className="mr-auto" disabled={!url || !token} onClick={runTest}>测试连接</Btn>
          <Btn onClick={onClose}>取消</Btn>
          <Btn
            kind="primary"
            disabled={!test?.ok}
            title={!test?.ok ? '先测试连接' : undefined}
            onClick={() => (actions.addPlatform({ id: `p${Date.now()}`, type, url: url.trim().replace(/\/$/, ''), tokenTail: token.slice(-4), addedAt: TODAY }), onClose())}
          >
            保存
          </Btn>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <span className="wb-label">类型</span>
          <div role="radiogroup" className="flex w-fit rounded-[2px] border border-line">
            {(['GitLab', 'GitHub'] as const).map((t) => (
              <button key={t} type="button" role="radio" aria-checked={type === t} onClick={() => (setType(t), reset())} className={`h-7 px-4 ${type === t ? 'bg-ink font-semibold text-white' : 'hover:bg-[var(--wb-hover-row)]'}`}>
                {t}
              </button>
            ))}
          </div>
        </div>
        <Field label="实例地址">
          <Input value={url} onChange={(e) => (setUrl(e.target.value), reset())} className="font-mono" placeholder="https://gitlab.corp.example" />
        </Field>
        <Field label="只读服务令牌">
          <Input type="password" value={token} onChange={(e) => (setToken(e.target.value), reset())} className="font-mono" />
        </Field>
        <p className={`text-[length:var(--wb-text-sm)] ${test ? (test.ok ? 'font-semibold' : 'font-semibold text-accent') : 'text-ink-3'}`}>
          {test ? test.msg : '保存前必须测试连接。修改地址或令牌后需要重新测试。'}
        </p>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- 用户

export function CreateUserDialog({ onClose }: { onClose: () => void }) {
  const [f, setF] = useState<LocalUserInput>({ username: '', displayName: '', email: '', dept: '', password: '' });
  const [tried, setTried] = useState(false);
  const errors = validateLocalUser(f);
  const show = (k: keyof LocalUserInput) => (tried ? errors[k] : undefined);
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.createLocalUser(f);
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={520}
      title="新建本地用户"
      description="给没有 LDAP 身份的人（例如外包同事）用。新用户只有一个本地账号密码登录身份，之后可以再绑定 LDAP。"
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="primary" onClick={submit}>创建用户</Btn>
        </>
      }
    >
      <div className="grid grid-cols-2 gap-x-5 gap-y-4">
        <Field label="用户名" error={show('username')} hint="也是本地登录名">
          <Input value={f.username} onChange={(e) => setF({ ...f, username: e.target.value.trim() })} invalid={!!show('username')} autoFocus />
        </Field>
        <Field label="显示名" error={show('displayName')}>
          <Input value={f.displayName} onChange={(e) => setF({ ...f, displayName: e.target.value })} invalid={!!show('displayName')} />
        </Field>
        <Field label="邮箱（可选）" error={show('email')}>
          <Input value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} invalid={!!show('email')} />
        </Field>
        <Field label="部门（可选）">
          <Input value={f.dept} onChange={(e) => setF({ ...f, dept: e.target.value })} />
        </Field>
        <Field label="初始密码" error={show('password')} hint="至少 8 位，线下告知本人">
          <Input type="password" value={f.password} onChange={(e) => setF({ ...f, password: e.target.value })} invalid={!!show('password')} />
        </Field>
      </div>
    </Dialog>
  );
}

export function DisableUserDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const s = useStore();
  const blocking = soleOwnerOf(s, user.username);
  if (blocking.length)
    return (
      <Dialog onClose={onClose} title={`不能禁用 ${user.displayName}`} description={`下面的应用只有 ${user.displayName} 一个 Owner。先请该应用的 Owner 添加另一位 Owner，再来禁用。`} footer={<Btn kind="primary" onClick={onClose}>知道了</Btn>}>
        <ul className="border-y border-ink">
          {blocking.map((a) => (
            <li key={a.key} className="flex items-baseline justify-between border-b border-line-2 py-2 last:border-b-0">
              <span>
                <span className="font-semibold">{a.name}</span>
                <Mono className="ml-2 text-ink-2">{a.key}</Mono>
              </span>
              <LinkBtn onClick={() => (onClose(), navigate(`/apps/${a.key}`))}>打开应用</LinkBtn>
            </li>
          ))}
        </ul>
      </Dialog>
    );
  return (
    <Dialog
      onClose={onClose}
      title={`禁用 ${user.displayName}`}
      description="禁用后该用户不能用任何登录身份登录，也不能再被设为 Owner 或 PE。用户信息和历史记录保留，可以随时重新启用。"
      footer={
        <>
          <Btn onClick={onClose}>取消</Btn>
          <Btn kind="solid-danger" onClick={() => (actions.setUserStatus(user.username, '已禁用'), onClose())}>禁用</Btn>
        </>
      }
    >
      <p className="text-ink-2">
        登录身份：{user.identities.map((i) => `${IDENTITY_LABEL[i.type]} ${i.login}`).join('、')}
      </p>
    </Dialog>
  );
}
