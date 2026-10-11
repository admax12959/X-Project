import { useState } from 'react';
import {
  actions,
  appVersions,
  boundBySelf,
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
  versionById,
  type AppInput,
  type Application,
  type CodePlatform,
  type IdentityType,
  type Line,
  type LineStatus,
  type LocalUserInput,
  type User,
} from '../../data';
import { useT, type Msg } from '../../i18n';
import { navigate } from '../../router';
import { Btn, Dialog, Field, Input, LinkBtn, Mono, Select, Textarea, useLineName } from './ui';

// ---------------------------------------------------------------- people picker

function PeoplePicker({ label, value, onChange, error, role }: { label: string; value: string[]; onChange: (v: string[]) => void; error?: string; role: 'owner' | 'pe' }) {
  const { t, tx } = useT();
  const s = useStore();
  const options = s.users.filter((u) => u.status === 'Enabled' && !value.includes(u.username));
  return (
    <div role="group" aria-label={label}>
      <span className="cs-label">{label}</span>
      <div className="cs-picker" style={error ? { borderColor: 'var(--c-danger)' } : undefined}>
        {value.length === 0 && <div className="cs-picker-row cs-muted">{t('common.none')}</div>}
        {value.map((u) => (
          <div key={u} className="cs-picker-row">
            <span>
              {userName(u)} <span className="cs-muted cs-small">{u}</span>
            </span>
            <LinkBtn className="cs-small" aria-label={`${t('console.remove')} ${userName(u)}`} onClick={() => onChange(value.filter((x) => x !== u))}>
              {t('console.remove')}
            </LinkBtn>
          </div>
        ))}
        <div className="cs-picker-add">
          <select
            className="cs-select"
            aria-label={`${t('console.add')} ${t(`role.${role}`)}`}
            value=""
            disabled={options.length === 0}
            onChange={(e) => e.target.value && onChange([...value, e.target.value])}
          >
            <option value="">{options.length ? `${t('console.add')} ${t(`role.${role}`)}…` : t('console.noUsersToAdd')}</option>
            {options.map((u) => (
              <option key={u.username} value={u.username}>
                {tx(u.displayName)} ({u.username}){u.dept ? ` · ${tx(u.dept)}` : ''}
              </option>
            ))}
          </select>
        </div>
      </div>
      {error ? <span className="cs-error">{error}</span> : <span className="cs-hint">{t('console.enabledOnly')}</span>}
    </div>
  );
}

// ---------------------------------------------------------------- create application

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const { t, tm } = useT();
  const s = useStore();
  const [f, setF] = useState<AppInput>({ key: '', name: '', description: '', platformId: s.platforms[0]?.id ?? '', repoPath: '', projectId: '', mainBranch: 'main', owners: [], pes: [] });
  const [verifyMsg, setVerifyMsg] = useState<Msg | null>(null);
  const [tried, setTried] = useState(false);
  const set = <K extends keyof AppInput>(k: K, v: AppInput[K]) => setF((p) => ({ ...p, [k]: v }));
  const setRepo = (patch: Partial<AppInput>) => {
    setF((p) => ({ ...p, ...patch, projectId: '' }));
    setVerifyMsg(null);
  };
  const errors = validateApp(f, 'create');
  const keyErr = f.key || tried ? errors.key : undefined;
  const show = (k: keyof AppInput) => (tried ? errors[k] : undefined);
  const err = (m?: Msg) => (m ? tm(m) : undefined);
  const verify = () => {
    const r = verifyRepo(f.platformId, f.repoPath);
    if (r.ok) {
      set('projectId', r.projectId);
      setVerifyMsg(null);
    } else {
      set('projectId', '');
      setVerifyMsg(r.message);
    }
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
      width={700}
      title={t('console.createApp.title')}
      description={t('console.createApp.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t('console.createApp.submit')}
          </Btn>
        </>
      }
    >
      <div className="cs-form-grid">
        <Field label={t('term.applicationKey')} error={err(keyErr)} hint={t('console.createApp.keyHint')}>
          <Input value={f.key} onChange={(e) => set('key', e.target.value.trim())} className="cs-mono" placeholder="billing-core" invalid={!!keyErr} autoFocus />
        </Field>
        <Field label={t('term.name')} error={err(show('name'))}>
          <Input value={f.name} onChange={(e) => set('name', e.target.value)} invalid={!!show('name')} />
        </Field>
        <Field label={t('term.description')} className="span2">
          <Input value={f.description} onChange={(e) => set('description', e.target.value)} />
        </Field>
        <div className="span2 cs-optional" style={{ borderTop: '1px solid var(--c-sep)' }}>
          <h3 className="cs-h3" style={{ marginBottom: 12 }}>
            {t('term.repositoryBinding')}
          </h3>
          <div style={{ display: 'grid', gridTemplateColumns: '240px minmax(0,1fr) auto', gap: 12, alignItems: 'start' }}>
            <Field label={t('term.codePlatform')} error={err(show('platformId'))}>
              <Select value={f.platformId} onChange={(e) => setRepo({ platformId: e.target.value })}>
                {s.platforms.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.type} · {p.url.replace(/^https?:\/\//, '')}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label={t('console.repoPath')} error={err(verifyMsg ?? show('repoPath'))}>
              <Input value={f.repoPath} onChange={(e) => setRepo({ repoPath: e.target.value })} className="cs-mono" placeholder="group/repo" invalid={!!(verifyMsg ?? show('repoPath'))} />
            </Field>
            <div>
              <span className="cs-label" style={{ visibility: 'hidden' }}>
                .
              </span>
              <Btn onClick={verify} disabled={!f.repoPath.trim()}>
                {t('console.verify')}
              </Btn>
            </div>
          </div>
          <div className="cs-small" style={{ marginTop: 8 }} role="status">
            {f.projectId ? (
              <span>
                <b>{t('console.repoFound')}</b> <span className="cs-muted">{t('console.repoFoundDetail', { platform: platform?.type ?? '', id: f.projectId })}</span>
              </span>
            ) : (
              <span className="cs-muted">{t('console.verifyHint')}</span>
            )}
          </div>
        </div>
        <Field label={t('term.mainBranch')} error={err(show('mainBranch'))}>
          <Input value={f.mainBranch} onChange={(e) => set('mainBranch', e.target.value)} className="cs-mono" invalid={!!show('mainBranch')} />
        </Field>
        <div />
        <PeoplePicker label={t('term.owners')} role="owner" value={f.owners} onChange={(v) => set('owners', v)} error={err(show('owners'))} />
        <PeoplePicker label={t('term.pes')} role="pe" value={f.pes} onChange={(v) => set('pes', v)} />
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- edit application

export function EditAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const { t, tx, tm } = useT();
  const s = useStore();
  const [f, setF] = useState({ name: tx(app.name), description: tx(app.description), mainBranch: app.mainBranch, owners: app.owners, pes: app.pes });
  const [tried, setTried] = useState(false);
  const input: AppInput = { ...f, key: app.key, platformId: app.repo.platformId, repoPath: app.repo.path, projectId: app.repo.projectId };
  const errors = validateApp(input, 'edit');
  const show = (k: keyof AppInput) => (tried && errors[k] ? tm(errors[k]!) : undefined);
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
      width={680}
      title={t('console.editApp.title')}
      description={t('console.editApp.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t('common.save')}
          </Btn>
        </>
      }
    >
      <div className="cs-form-grid">
        <Field label={t('term.applicationKey')} hint={t('console.immutable')}>
          <Input readOnly value={app.key} className="cs-mono" />
        </Field>
        <Field label={t('term.repository')} hint={`${platform?.type ?? ''} · ${t('term.projectId')} ${app.repo.projectId} · ${t('console.immutable')}`}>
          <Input readOnly value={app.repo.path} className="cs-mono" />
        </Field>
        <Field label={t('term.name')} error={show('name')}>
          <Input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} invalid={!!show('name')} autoFocus />
        </Field>
        <Field label={t('term.mainBranch')} error={show('mainBranch')}>
          <Input value={f.mainBranch} onChange={(e) => setF({ ...f, mainBranch: e.target.value })} className="cs-mono" invalid={!!show('mainBranch')} />
        </Field>
        <Field label={t('term.description')} className="span2">
          <Textarea rows={2} value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} />
        </Field>
        <PeoplePicker label={t('term.owners')} role="owner" value={f.owners} onChange={(v) => setF({ ...f, owners: v })} error={show('owners')} />
        <PeoplePicker label={t('term.pes')} role="pe" value={f.pes} onChange={(v) => setF({ ...f, pes: v })} />
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- delete application

export function DeleteAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const [typed, setTyped] = useState('');
  const lineCount = s.lines.filter((l) => l.appKey === app.key && !l.isMain).length;
  const verCount = appVersions(s, app.key).length;
  return (
    <Dialog
      onClose={onClose}
      title={t('console.deleteApp.title', { name: tx(app.name) })}
      description={t('console.deleteApp.desc', { lines: t('n.lines', { n: lineCount }), versions: t('n.versions', { n: verCount }) })}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn
            kind="danger"
            disabled={typed !== app.key}
            onClick={() => {
              onClose();
              navigate('/');
              actions.deleteApp(app.key);
            }}
          >
            {t('console.deleteApp.submit')}
          </Btn>
        </>
      }
    >
      <Field label={t('console.deleteApp.confirm', { key: app.key })}>
        <Input value={typed} onChange={(e) => setTyped(e.target.value)} className="cs-mono" autoFocus />
      </Field>
    </Dialog>
  );
}

// ---------------------------------------------------------------- fork

export function ForkDialog({ appKey, sourceVersionId, onClose, onCreated }: { appKey: string; sourceVersionId: string; onClose: () => void; onCreated: (lineId: string) => void }) {
  const { t, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const app = s.apps.find((a) => a.key === appKey)!;
  const [src, setSrc] = useState(sourceVersionId);
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const b = branch.trim();
  const lines = s.lines.filter((l) => l.appKey === appKey);
  const taken = lines.find((l) => (l.isMain ? app.mainBranch : l.branch) === b) ?? lines.find((l) => l.id === `${appKey}:${b}`);
  const errors = {
    src: !src ? t('console.fork.err.src') : undefined,
    name: !name.trim() ? t('console.fork.err.name') : undefined,
    branch: !b ? t('console.fork.err.branch') : taken ? t('console.fork.err.branchTaken', { line: lineName(taken) }) : undefined,
    reason: !reason.trim() ? t('console.fork.err.reason') : undefined,
  };
  const show = (k: keyof typeof errors) => (k === 'branch' && taken ? errors.branch : tried ? errors[k] : undefined);
  const srcVer = versionById(src);
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
      width={620}
      title={t('console.fork.title')}
      description={t('console.fork.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t('term.fork')}
          </Btn>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 16 }}>
        <Field
          label={t('term.sourceVersion')}
          error={show('src')}
          hint={srcVer && srcLine ? t('console.fork.srcHint', { line: lineName(srcLine), date: fmtDate(srcVer.releasedAt), status: t(`status.${srcLine.status}`) }) : undefined}
        >
          <Select value={src} onChange={(e) => setSrc(e.target.value)} invalid={!!show('src')}>
            <option value="">{t('console.fork.pickVersion')}</option>
            {lineageOrder(s, appKey).map(({ line }) => {
              const vs = lineVersions(s, line.id);
              if (!vs.length) return null;
              return (
                <optgroup key={line.id} label={`${lineName(line)} (${t(`status.${line.status}`)})`}>
                  {vs
                    .slice()
                    .reverse()
                    .map((v) => (
                      <option key={v.id} value={v.id}>
                        {v.number} · {fmtDate(v.releasedAt)}
                      </option>
                    ))}
                </optgroup>
              );
            })}
          </Select>
        </Field>
        <div className="cs-form-grid">
          <Field label={t('console.fork.lineName')} error={show('name')}>
            <Input value={name} onChange={(e) => setName(e.target.value)} invalid={!!show('name')} autoFocus />
          </Field>
          <Field label={t('term.branch')} error={show('branch')} hint={t('console.fork.branchHint')}>
            <Input value={branch} onChange={(e) => setBranch(e.target.value)} className="cs-mono" placeholder="cust/acme" invalid={!!show('branch')} />
          </Field>
        </div>
        <Field label={t('term.forkReason')} error={show('reason')} hint={t('console.fork.reasonHint')}>
          <Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} invalid={!!show('reason')} />
        </Field>
        <details className="cs-optional">
          <summary>{t('common.optional')}</summary>
          <div className="cs-form-grid" style={{ marginTop: 12 }}>
            <Field label={t('term.plannedEnd')}>
              <Input type="date" value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
            </Field>
            <Field label={t('term.note')}>
              <Input value={note} onChange={(e) => setNote(e.target.value)} />
            </Field>
          </div>
        </details>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- register version

export function RegisterVersionDialog({ appKey, initialLineId, onClose, onDone }: { appKey: string; initialLineId: string; onClose: () => void; onDone?: (id: string) => void }) {
  const { t, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const app = s.apps.find((a) => a.key === appKey)!;
  const order = lineageOrder(s, appKey).map((x) => x.line);
  const [lineId, setLineId] = useState(initialLineId);
  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<'tag' | 'commit'>('tag');
  const [ref, setRef] = useState('');
  const [releasedAt, setReleasedAt] = useState(TODAY);
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const line = lineById(lineId)!;
  const frozen = line.status === 'Frozen';
  const retired = line.status === 'Retired';
  const n = number.trim();
  const clash = n ? appVersions(s, appKey).find((v) => v.number === n) : undefined;
  const clashLine = clash ? lineById(clash.lineId) : undefined;
  const errors = {
    number: !n ? t('console.reg.err.number') : clash && clashLine ? t('console.reg.err.numberTaken', { line: lineName(clashLine), date: fmtDate(clash.releasedAt) }) : undefined,
    ref: !ref.trim() ? t('console.reg.err.ref') : undefined,
    releasedAt: !releasedAt ? t('console.reg.err.date') : undefined,
    note: frozen && !note.trim() ? t('console.reg.err.noteFrozen') : undefined,
  };
  const show = (k: keyof typeof errors) => (k === 'number' && clash ? errors.number : tried ? errors[k] : undefined);
  const last = lineVersions(s, line.id).at(-1);

  const submit = () => {
    setTried(true);
    if (retired || Object.values(errors).some(Boolean)) return;
    actions.registerVersion({ lineId: line.id, number: n, ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onDone?.(n);
    onClose();
  };

  return (
    <Dialog
      onClose={onClose}
      width={560}
      title={t('console.reg.title')}
      description={
        <>
          {t('console.reg.desc')} <Mono>{line.isMain ? app.mainBranch : line.branch}</Mono>
          {last ? ` · ${t('console.reg.last', { number: last.number, date: fmtDate(last.releasedAt) })}` : ` · ${t('console.reg.noVersions')}`}
        </>
      }
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} disabled={retired} title={retired ? t('console.retiredNoRegister') : undefined}>
            {t('console.reg.submit')}
          </Btn>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 16 }}>
        <Field label={t('term.line')} error={retired ? t('console.retiredNoRegister') : undefined}>
          <Select value={lineId} onChange={(e) => setLineId(e.target.value)}>
            {order.map((l) => (
              <option key={l.id} value={l.id} disabled={l.status === 'Retired'}>
                {lineName(l)} ({t(`status.${l.status}`)}){l.status === 'Retired' ? ` – ${t('console.retiredNoRegisterShort')}` : ''}
              </option>
            ))}
          </Select>
        </Field>
        {frozen && <p className="cs-small" style={{ margin: 0, fontWeight: 600 }}>{t('console.reg.frozenNotice')}</p>}
        <Field label={t('term.version')} error={show('number')} hint={t('console.reg.numberHint')}>
          <Input value={number} onChange={(e) => setNumber(e.target.value)} className="cs-mono" invalid={!!show('number')} autoFocus />
        </Field>
        <div>
          <span className="cs-label">{t('term.ref')}</span>
          <div style={{ display: 'flex', gap: 8 }}>
            <div role="radiogroup" aria-label={t('term.ref')} className="cs-seg" style={{ flex: 'none' }}>
              {(['tag', 'commit'] as const).map((k) => (
                <button key={k} type="button" role="radio" aria-checked={kind === k} onClick={() => setKind(k)}>
                  {t(`ref.${k}`)}
                </button>
              ))}
            </div>
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="cs-mono" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} invalid={!!show('ref')} aria-label={t(`ref.${kind}`)} />
          </div>
          {show('ref') && <span className="cs-error">{show('ref')}</span>}
        </div>
        <Field label={t('term.releasedAt')} error={show('releasedAt')}>
          <Input type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} style={{ width: 180 }} />
        </Field>
        <Field label={frozen ? `${t('term.note')} (${t('common.required')})` : t('term.note')} error={show('note')}>
          <Textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} invalid={!!show('note')} />
        </Field>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- line status

export const NEXT_STATUS: Partial<Record<LineStatus, LineStatus>> = { Active: 'Frozen', Frozen: 'Retired' };
export const nextStatus = (l: Line) => (l.isMain ? undefined : NEXT_STATUS[l.status]);

export function StatusDialog({ line, to, onClose }: { line: Line; to: LineStatus; onClose: () => void }) {
  const { t } = useT();
  const lineName = useLineName();
  return (
    <Dialog
      onClose={onClose}
      title={t('console.status.title', { line: lineName(line), status: t(`status.${to}`) })}
      description={t(to === 'Frozen' ? 'console.status.toFrozen' : 'console.status.toRetired')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn
            kind={to === 'Retired' ? 'danger' : 'primary'}
            onClick={() => {
              actions.setLineStatus(line.id, to);
              onClose();
            }}
          >
            {t('console.status.submit', { status: t(`status.${to}`) })}
          </Btn>
        </>
      }
    />
  );
}

// ---------------------------------------------------------------- deployment info

export function DeployDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const { t, tx } = useT();
  const [text, setText] = useState(tx(app.deployInfo));
  return (
    <Dialog
      onClose={onClose}
      width={600}
      title={t('console.deploy.title')}
      description={t('console.deploy.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn
            kind="primary"
            onClick={() => {
              actions.setDeployInfo(app.key, text);
              onClose();
            }}
          >
            {t('common.save')}
          </Btn>
        </>
      }
    >
      <Field label={t('term.deployInfo')}>
        <Textarea rows={7} value={text} onChange={(e) => setText(e.target.value)} autoFocus />
      </Field>
    </Dialog>
  );
}

// ---------------------------------------------------------------- login identities

/** `by` is who binds: the user themself (Settings) or an Administrator. */
export function BindDialog({ user, type, byAdmin, onClose }: { user: User; type: IdentityType; byAdmin: boolean; onClose: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const [login, setLogin] = useState(type === 'local' ? user.username : '');
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [tried, setTried] = useState(false);
  const ldap = type === 'ldap';
  const errors = {
    login: !login.trim() ? t(ldap ? 'console.bind.err.ldapUser' : 'console.bind.err.localUser') : undefined,
    pw: !pw ? t('console.bind.err.password') : ldap && pw.length < 6 ? t('console.bind.err.ldapAuth') : !ldap && pw.length < 8 ? t('err.passwordShort') : undefined,
    pw2: !ldap && pw !== pw2 ? t('console.bind.err.mismatch') : undefined,
  };
  const show = (k: keyof typeof errors) => (tried ? errors[k] : undefined);
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy: byAdmin ? { key: 'boundBy.admin', params: { name: userName(s.me) } } : boundBySelf() });
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={480}
      title={t('console.bind.title', { type: t(`identity.${type}`) })}
      description={t(ldap ? 'console.bind.descLdap' : 'console.bind.descLocal', { name: tx(user.displayName) })}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t(ldap ? 'console.bind.submitLdap' : 'console.bind.submitLocal')}
          </Btn>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 16 }}>
        <Field label={t(ldap ? 'console.bind.ldapUser' : 'console.bind.localUser')} error={show('login')}>
          <Input value={login} onChange={(e) => setLogin(e.target.value)} invalid={!!show('login')} autoFocus />
        </Field>
        <Field label={t('console.bind.password')} error={show('pw')} hint={ldap ? t('console.bind.ldapRule') : undefined}>
          <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} invalid={!!show('pw')} />
        </Field>
        {!ldap && (
          <Field label={t('console.bind.confirm')} error={show('pw2')}>
            <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} invalid={!!show('pw2')} />
          </Field>
        )}
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- code platform

export function PlatformDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const s = useStore();
  const [type, setType] = useState<CodePlatform['type']>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [tried, setTried] = useState(false);
  const u = url.trim().replace(/\/$/, '');
  const errors = {
    url: !/^https?:\/\/[\w.-]+(:\d+)?$/.test(u) ? t('console.platform.err.url') : s.platforms.some((p) => p.url === u) ? t('console.platform.err.urlTaken') : undefined,
    token: token.length < 8 ? t('console.platform.err.token', { type }) : undefined,
  };
  const show = (k: keyof typeof errors) => (tried ? errors[k] : undefined);
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.addPlatform({ id: `p${Date.now()}`, type, url: u, tokenTail: token.slice(-4), addedAt: TODAY });
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={520}
      title={t('console.platform.title')}
      description={t('console.platform.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t('common.save')}
          </Btn>
        </>
      }
    >
      <div style={{ display: 'grid', gap: 16 }}>
        <div>
          <span className="cs-label">{t('console.platform.type')}</span>
          <div role="radiogroup" aria-label={t('console.platform.type')} className="cs-seg">
            {(['GitLab', 'GitHub'] as const).map((x) => (
              <button key={x} type="button" role="radio" aria-checked={type === x} onClick={() => setType(x)}>
                {x}
              </button>
            ))}
          </div>
        </div>
        <Field label={t('console.platform.url')} error={show('url')}>
          <Input value={url} onChange={(e) => setUrl(e.target.value)} className="cs-mono" placeholder="https://gitlab.corp.example" invalid={!!show('url')} autoFocus />
        </Field>
        <Field label={t('console.platform.token')} error={show('token')} hint={t('console.platform.tokenHint')}>
          <Input type="password" value={token} onChange={(e) => setToken(e.target.value)} className="cs-mono" invalid={!!show('token')} />
        </Field>
      </div>
    </Dialog>
  );
}

// ---------------------------------------------------------------- users

export function CreateUserDialog({ onClose }: { onClose: () => void }) {
  const { t, tm } = useT();
  const [f, setF] = useState<LocalUserInput>({ username: '', displayName: '', email: '', dept: '', password: '' });
  const [tried, setTried] = useState(false);
  const errors = validateLocalUser(f);
  const show = (k: keyof LocalUserInput) => (tried && errors[k] ? tm(errors[k]!) : undefined);
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.createLocalUser(f);
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={540}
      title={t('console.createUser.title')}
      description={t('console.createUser.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit}>
            {t('console.createUser.submit')}
          </Btn>
        </>
      }
    >
      <div className="cs-form-grid">
        <Field label={t('console.username')} error={show('username')} hint={t('console.createUser.usernameHint')}>
          <Input value={f.username} onChange={(e) => setF({ ...f, username: e.target.value.trim() })} invalid={!!show('username')} autoFocus />
        </Field>
        <Field label={t('term.name')} error={show('displayName')}>
          <Input value={f.displayName} onChange={(e) => setF({ ...f, displayName: e.target.value })} invalid={!!show('displayName')} />
        </Field>
        <Field label={`${t('console.email')} (${t('common.optional')})`} error={show('email')}>
          <Input value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} invalid={!!show('email')} />
        </Field>
        <Field label={`${t('console.dept')} (${t('common.optional')})`}>
          <Input value={f.dept} onChange={(e) => setF({ ...f, dept: e.target.value })} />
        </Field>
        <Field label={t('console.createUser.password')} error={show('password')} hint={t('console.createUser.passwordHint')}>
          <Input type="password" value={f.password} onChange={(e) => setF({ ...f, password: e.target.value })} invalid={!!show('password')} />
        </Field>
      </div>
    </Dialog>
  );
}

export function DisableUserDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const blocking = soleOwnerOf(s, user.username);
  const name = tx(user.displayName);
  if (blocking.length)
    return (
      <Dialog
        onClose={onClose}
        title={t('console.disable.blockedTitle', { name })}
        description={t('console.disable.blockedDesc', { name })}
        footer={
          <Btn kind="primary" onClick={onClose}>
            {t('common.gotIt')}
          </Btn>
        }
      >
        <ul style={{ listStyle: 'none', margin: 0, padding: 0, borderTop: '1px solid var(--c-sep)' }}>
          {blocking.map((a) => (
            <li key={a.key} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: '8px 0', borderBottom: '1px solid var(--c-sep)' }}>
              <span>
                {tx(a.name)} <Mono className="cs-muted">{a.key}</Mono>
              </span>
              <LinkBtn
                onClick={() => {
                  onClose();
                  navigate(`/apps/${a.key}`);
                }}
              >
                {t('console.disable.openApp')}
              </LinkBtn>
            </li>
          ))}
        </ul>
      </Dialog>
    );
  return (
    <Dialog
      onClose={onClose}
      title={t('console.disable.title', { name })}
      description={t('console.disable.desc')}
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn
            kind="danger"
            onClick={() => {
              actions.setUserStatus(user.username, 'Disabled');
              onClose();
            }}
          >
            {t('console.disable.submit')}
          </Btn>
        </>
      }
    />
  );
}
