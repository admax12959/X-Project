import { useState } from 'react';
import {
  actions,
  appVersions,
  lineById,
  lineageOrder,
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
  type IdentityType,
  type Line,
  type LocalUserInput,
  type User,
} from '../../data';
import { useT } from '../../i18n';
import type { Msg } from '../../i18n';
import { appPath, navigate } from '../../router';
import { A, Btn, ConfirmDialog, Field, Input, Modal, Mono, PeoplePicker, platformLabel, Select, Textarea } from './ui';

export const lineHref = (appKey: string, lineId: string) => `${appPath(appKey, 'lineage')}?line=${encodeURIComponent(lineId)}`;
export const versionHref = (appKey: string, versionId: string) => `${appPath(appKey, 'versions')}?v=${encodeURIComponent(versionId)}`;

// ---------------------------------------------------------------- create application

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const { t, tm } = useT();
  const s = useStore();
  const [f, setF] = useState<AppInput>({ key: '', name: '', description: '', platformId: s.platforms[0]?.id ?? '', repoPath: '', projectId: '', mainBranch: 'main', owners: [], pes: [] });
  const [verifyMsg, setVerifyMsg] = useState<Msg | null>(null);
  const [tried, setTried] = useState(false);
  const set = <K extends keyof AppInput>(k: K, v: AppInput[K]) => setF((p) => ({ ...p, [k]: v }));
  // changing the repository invalidates an earlier verification
  const setRepo = (patch: Partial<AppInput>) => {
    setF((p) => ({ ...p, ...patch, projectId: '' }));
    setVerifyMsg(null);
  };
  const errors = validateApp(f, 'create');
  const show = (k: keyof AppInput) => (tried ? errors[k] : undefined);
  const keyErr = f.key ? errors.key : show('key');
  const msg = (m?: Msg | null) => (m ? tm(m) : undefined);
  const platform = s.platforms.find((p) => p.id === f.platformId);

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
    navigate(appPath(f.key));
  };
  return (
    <Modal
      title={t('catalog.createApp')}
      description={t('catalog.createAppHint')}
      onClose={onClose}
      width={680}
      dataCg="create-app"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} data-cg="create-app-submit">
            {t('catalog.createApp')}
          </Btn>
        </>
      }
    >
      <div className="cg-form">
        <Field label={t('term.applicationKey')} error={msg(keyErr)} hint={t('catalog.keyHint')}>
          <Input value={f.key} onChange={(e) => set('key', e.target.value.trim())} className="cg-mono" invalid={!!keyErr} autoFocus data-cg="f-key" />
        </Field>
        <Field label={t('term.name')} error={msg(show('name'))}>
          <Input value={f.name} onChange={(e) => set('name', e.target.value)} invalid={!!show('name')} data-cg="f-name" />
        </Field>
        <Field label={t('term.description')} className="full">
          <Input value={f.description} onChange={(e) => set('description', e.target.value)} data-cg="f-desc" />
        </Field>
        <Field label={t('term.codePlatform')} error={msg(show('platformId'))}>
          <Select value={f.platformId} onChange={(e) => setRepo({ platformId: e.target.value })} data-cg="f-platform">
            {s.platforms.map((p) => (
              <option key={p.id} value={p.id}>
                {platformLabel(p)}
              </option>
            ))}
          </Select>
        </Field>
        <div>
          <Field label={t('catalog.repoPath')} error={msg(verifyMsg ?? show('repoPath'))}>
            <div style={{ display: 'flex', gap: 8 }}>
              <Input value={f.repoPath} onChange={(e) => setRepo({ repoPath: e.target.value })} className="cg-mono" placeholder="group/repo" invalid={!!(verifyMsg ?? show('repoPath'))} data-cg="f-repo" />
              <Btn onClick={verify} disabled={!f.repoPath.trim()} data-cg="verify">
                {t('catalog.verify')}
              </Btn>
            </div>
          </Field>
        </div>
        <div className="full cg-note" data-cg="verify-result" aria-live="polite">
          {f.projectId ? (
            <span>
              {t('catalog.repoFound', { platform: platform?.type ?? '' })} {t('term.projectId')} <Mono>{f.projectId}</Mono>
            </span>
          ) : (
            <span className="cg-grey">{t('catalog.verifyHint')}</span>
          )}
        </div>
        <Field label={t('term.mainBranch')} error={msg(show('mainBranch'))} hint={t('catalog.mainBranchHint')}>
          <Input value={f.mainBranch} onChange={(e) => set('mainBranch', e.target.value)} className="cg-mono" invalid={!!show('mainBranch')} data-cg="f-main" />
        </Field>
        <div />
        <PeoplePicker label={t('term.owners')} value={f.owners} onChange={(v) => set('owners', v)} error={msg(show('owners'))} dataCg="pick-owners" />
        <PeoplePicker label={t('term.pes')} value={f.pes} onChange={(v) => set('pes', v)} dataCg="pick-pes" />
      </div>
    </Modal>
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
    <Modal
      title={t('catalog.editApp')}
      description={t('catalog.editAppHint')}
      onClose={onClose}
      width={680}
      dataCg="edit-app"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} data-cg="edit-app-submit">
            {t('common.save')}
          </Btn>
        </>
      }
    >
      <div className="cg-form">
        <Field label={t('term.applicationKey')} hint={t('catalog.immutable')}>
          <Input readOnly value={app.key} className="cg-mono" />
        </Field>
        <Field label={t('term.repository')} hint={`${platformLabel(platform)} · ${t('term.projectId')} ${app.repo.projectId} · ${t('catalog.immutable')}`}>
          <Input readOnly value={app.repo.path} className="cg-mono" />
        </Field>
        <Field label={t('term.name')} error={show('name')}>
          <Input value={f.name} onChange={(e) => setF({ ...f, name: e.target.value })} invalid={!!show('name')} autoFocus data-cg="f-name" />
        </Field>
        <Field label={t('term.mainBranch')} error={show('mainBranch')}>
          <Input value={f.mainBranch} onChange={(e) => setF({ ...f, mainBranch: e.target.value })} className="cg-mono" invalid={!!show('mainBranch')} data-cg="f-main" />
        </Field>
        <Field label={t('term.description')} className="full">
          <Textarea rows={2} value={f.description} onChange={(e) => setF({ ...f, description: e.target.value })} data-cg="f-desc" />
        </Field>
        <PeoplePicker label={t('term.owners')} value={f.owners} onChange={(v) => setF({ ...f, owners: v })} error={show('owners')} dataCg="pick-owners" />
        <PeoplePicker label={t('term.pes')} value={f.pes} onChange={(v) => setF({ ...f, pes: v })} dataCg="pick-pes" />
      </div>
    </Modal>
  );
}

// ---------------------------------------------------------------- delete application

export function DeleteAppDialog({ app, onClose, onDeleted }: { app: Application; onClose: () => void; onDeleted?: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const lineCount = s.lines.filter((l) => l.appKey === app.key && !l.isMain).length;
  return (
    <ConfirmDialog
      danger
      title={t('catalog.deleteApp', { name: tx(app.name) })}
      description={t('catalog.deleteAppBody', { lines: lineCount, versions: appVersions(s, app.key).length })}
      confirmLabel={t('common.delete')}
      onClose={onClose}
      onConfirm={() => {
        onDeleted?.();
        navigate('/');
        actions.deleteApp(app.key);
        onClose();
      }}
    />
  );
}

// ---------------------------------------------------------------- fork

export function ForkDialog({ appKey, sourceVersionId, onClose, onCreated }: { appKey: string; sourceVersionId: string; onClose: () => void; onCreated: (lineId: string) => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const app = s.apps.find((a) => a.key === appKey)!;
  const [src, setSrc] = useState(sourceVersionId);
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const b = branch.trim();
  const taken = s.lines.find((l) => l.appKey === appKey && ((l.isMain ? app.mainBranch : l.branch) === b || l.id === `${appKey}:${b}`));
  const errors = {
    src: !src ? t('catalog.err.source') : undefined,
    name: !name.trim() ? t('catalog.err.lineName') : undefined,
    branch: !b ? t('catalog.err.branch') : taken ? t('catalog.err.branchTaken', { line: taken.isMain ? t('term.mainline') : tx(taken.name) }) : undefined,
    reason: !reason.trim() ? t('catalog.err.reason') : undefined,
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
    <Modal
      title={t('catalog.forkTitle')}
      description={t('catalog.forkHint')}
      onClose={onClose}
      width={620}
      dataCg="fork"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} data-cg="fork-submit">
            {t('term.fork')}
          </Btn>
        </>
      }
    >
      <div className="cg-form">
        <Field
          label={t('term.sourceVersion')}
          error={show('src')}
          className="full"
          hint={srcVer && srcLine ? `${srcLine.isMain ? t('term.mainline') : tx(srcLine.name)} · ${fmtDate(srcVer.releasedAt)}${srcLine.status === 'Retired' ? ` · ${t('catalog.forkFromRetired')}` : ''}` : t('catalog.sourceHint')}
        >
          <Select value={src} onChange={(e) => setSrc(e.target.value)} invalid={!!show('src')} data-cg="f-source">
            <option value="">{t('catalog.selectVersion')}</option>
            {lineageOrder(s, appKey).map(({ line }) => {
              const vs = lineVersions(s, line.id);
              if (!vs.length) return null;
              return (
                <optgroup key={line.id} label={`${line.isMain ? t('term.mainline') : tx(line.name)} (${t(`status.${line.status}`)})`}>
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
        <Field label={t('term.name')} error={show('name')}>
          <Input value={name} onChange={(e) => setName(e.target.value)} invalid={!!show('name')} autoFocus data-cg="f-name" />
        </Field>
        <Field label={t('term.branch')} error={show('branch')} hint={t('catalog.branchHint')}>
          <Input value={branch} onChange={(e) => setBranch(e.target.value)} className="cg-mono" placeholder="cust/acme" invalid={!!show('branch')} data-cg="f-branch" />
        </Field>
        <Field label={t('term.forkReason')} error={show('reason')} hint={t('catalog.reasonHint')} className="full">
          <Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} invalid={!!show('reason')} data-cg="f-reason" />
        </Field>
        <details className="cg-details full" data-cg="fork-optional">
          <summary>{t('common.optional')}</summary>
          <div className="cg-form" style={{ marginTop: 10 }}>
            <Field label={t('term.plannedEnd')}>
              <Input type="date" value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} data-cg="f-planned" />
            </Field>
            <Field label={t('term.note')}>
              <Input value={note} onChange={(e) => setNote(e.target.value)} data-cg="f-note" />
            </Field>
          </div>
        </details>
      </div>
    </Modal>
  );
}

// ---------------------------------------------------------------- register version

export function RegisterVersionDialog({ appKey, lineId, onClose, onDone }: { appKey: string; lineId: string; onClose: () => void; onDone?: (number: string) => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const [lid, setLid] = useState(lineId);
  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<'tag' | 'commit'>('tag');
  const [ref, setRef] = useState('');
  const [releasedAt, setReleasedAt] = useState(TODAY);
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const line = lineById(lid)!;
  const frozen = line.status === 'Frozen';
  const n = number.trim();
  const clash = n ? appVersions(s, appKey).find((v) => v.number === n) : undefined;
  const clashLine = clash ? lineById(clash.lineId) : undefined;
  const errors = {
    number: !n ? t('catalog.err.versionNumber') : clash ? t('catalog.err.versionTaken', { n, line: clashLine?.isMain ? t('term.mainline') : tx(clashLine?.name), date: fmtDate(clash.releasedAt) }) : undefined,
    ref: !ref.trim() ? t('catalog.err.ref', { kind: t(`ref.${kind}`) }) : undefined,
    releasedAt: !releasedAt ? t('catalog.err.date') : undefined,
    note: frozen && !note.trim() ? t('catalog.err.frozenNote') : undefined,
  };
  const show = (k: keyof typeof errors) => (k === 'number' && clash ? errors.number : tried ? errors[k] : undefined);
  const last = lineVersions(s, lid).at(-1);
  const retired = line.status === 'Retired';
  const app = s.apps.find((a) => a.key === appKey)!;

  const submit = () => {
    setTried(true);
    if (retired || Object.values(errors).some(Boolean)) return;
    actions.registerVersion({ lineId: lid, number: n, ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onDone?.(n);
    onClose();
  };
  return (
    <Modal
      title={t('catalog.registerTitle')}
      description={
        <>
          <Mono>{line.isMain ? app.mainBranch : line.branch}</Mono> · {last ? t('catalog.lastVersion', { n: last.number, date: fmtDate(last.releasedAt) }) : t('catalog.noVersionsYet')}. {t('catalog.manualRegister')}
        </>
      }
      onClose={onClose}
      width={560}
      dataCg="register"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} disabled={retired} data-cg="register-submit">
            {t('catalog.register')}
          </Btn>
        </>
      }
    >
      <div className="cg-form">
        <Field label={t('term.line')} className="full" error={retired ? t('catalog.retiredNoRegister') : undefined} hint={frozen ? t('catalog.frozenRegisterHint') : undefined}>
          <Select value={lid} onChange={(e) => setLid(e.target.value)} data-cg="f-line">
            {lineageOrder(s, appKey).map(({ line: l }) => (
              <option key={l.id} value={l.id}>
                {l.isMain ? t('term.mainline') : tx(l.name)} ({t(`status.${l.status}`)})
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t('term.version')} error={show('number')} hint={t('catalog.numberHint')} className="full">
          <Input value={number} onChange={(e) => setNumber(e.target.value)} className="cg-mono" invalid={!!show('number')} autoFocus data-cg="f-number" />
        </Field>
        <div className="full">
          <span className="cg-label">{t('term.ref')}</span>
          <div style={{ display: 'flex', gap: 8 }}>
            <div className="cg-seg" role="radiogroup" aria-label={t('term.ref')}>
              {(['tag', 'commit'] as const).map((k) => (
                <button key={k} type="button" role="radio" aria-checked={kind === k} onClick={() => setKind(k)} data-cg={`kind-${k}`}>
                  {t(`ref.${k}`)}
                </button>
              ))}
            </div>
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="cg-mono" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} invalid={!!show('ref')} aria-label={t(`ref.${kind}`)} data-cg="f-ref" />
          </div>
          {show('ref') && (
            <span className="cg-err" role="alert">
              {show('ref')}
            </span>
          )}
        </div>
        <Field label={t('term.releasedAt')} error={show('releasedAt')}>
          <Input type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} data-cg="f-date" />
        </Field>
        <Field label={t('term.note')} error={show('note')} optional={!frozen}>
          <Input value={note} onChange={(e) => setNote(e.target.value)} invalid={!!show('note')} data-cg="f-note" />
        </Field>
      </div>
    </Modal>
  );
}

// ---------------------------------------------------------------- line status

export function StatusDialog({ line, to, onClose }: { line: Line; to: 'Frozen' | 'Retired'; onClose: () => void }) {
  const { t, tx } = useT();
  return (
    <ConfirmDialog
      title={t('catalog.statusTitle', { line: tx(line.name), status: t(`status.${to}`) })}
      description={t(`catalog.statusBody.${to}`)}
      confirmLabel={t(`catalog.setStatus.${to}`)}
      danger={to === 'Retired'}
      onClose={onClose}
      onConfirm={() => {
        actions.setLineStatus(line.id, to);
        onClose();
      }}
    />
  );
}

// ---------------------------------------------------------------- login identities

const fakeLdapRejects = (pw: string) => pw === 'wrong';

export function BindIdentityDialog({ user, asAdmin, onClose }: { user: User; asAdmin?: boolean; onClose: () => void }) {
  const { t } = useT();
  const s = useStore();
  const free = (['ldap', 'local'] as IdentityType[]).filter((ty) => !user.identities.some((i) => i.type === ty));
  const [type, setType] = useState<IdentityType>(free[0] ?? 'local');
  const [login, setLogin] = useState(user.username);
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [tried, setTried] = useState(false);

  const needPw = !(asAdmin && type === 'ldap');
  const errors = {
    login: !login.trim() ? t('catalog.err.login') : type === 'local' && s.users.some((u) => u.username !== user.username && u.identities.some((i) => i.type === 'local' && i.login === login.trim())) ? t('err.usernameTaken') : undefined,
    pw: !needPw ? undefined : !pw ? t('catalog.err.passwordRequired') : type === 'local' && pw.length < 8 ? t('err.passwordShort') : type === 'ldap' && fakeLdapRejects(pw) ? t('catalog.err.ldapRejected') : undefined,
    pw2: type === 'local' && needPw && pw2 !== pw ? t('catalog.err.passwordMismatch') : undefined,
  };
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    const boundBy: Msg = asAdmin ? { key: 'boundBy.admin', params: { name: userName(s.me) } } : { key: 'boundBy.self' };
    actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy });
    onClose();
  };
  const show = (k: keyof typeof errors) => (tried ? errors[k] : undefined);
  return (
    <Modal
      title={t('catalog.bindTitle')}
      description={asAdmin ? t('catalog.bindAdminHint', { name: userName(user.username) }) : t('catalog.bindHint')}
      onClose={onClose}
      width={460}
      dataCg="bind"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} disabled={free.length === 0} data-cg="bind-submit">
            {asAdmin ? t('catalog.bind') : t('catalog.verifyAndBind')}
          </Btn>
        </>
      }
    >
      {free.length === 0 ? (
        <p className="cg-note">{t('catalog.allBound')}</p>
      ) : (
        <div className="cg-form" style={{ gridTemplateColumns: '1fr' }}>
          <Field label={t('catalog.identityType')}>
            <Select value={type} onChange={(e) => setType(e.target.value as IdentityType)} data-cg="f-idtype">
              {free.map((ty) => (
                <option key={ty} value={ty}>
                  {t(`identity.${ty}`)}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={type === 'ldap' ? t('catalog.ldapUsername') : t('catalog.localUsername')} error={show('login')}>
            <Input value={login} onChange={(e) => setLogin(e.target.value)} className="cg-mono" invalid={!!show('login')} data-cg="f-login" />
          </Field>
          {needPw && (
            <Field label={t('catalog.password')} error={show('pw')} hint={type === 'ldap' ? t('catalog.ldapDemoHint') : undefined}>
              <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} invalid={!!show('pw')} autoComplete="new-password" data-cg="f-pw" />
            </Field>
          )}
          {type === 'local' && needPw && (
            <Field label={t('catalog.confirmPassword')} error={show('pw2')}>
              <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} invalid={!!show('pw2')} autoComplete="new-password" data-cg="f-pw2" />
            </Field>
          )}
        </div>
      )}
    </Modal>
  );
}

// ---------------------------------------------------------------- users (admin)

export function SoleOwnerBlockedDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const apps = soleOwnerOf(s, user.username);
  return (
    <Modal
      title={t('catalog.cannotDisable', { name: userName(user.username) })}
      description={t('catalog.cannotDisableBody')}
      onClose={onClose}
      width={480}
      dataCg="blocked"
      footer={
        <Btn kind="primary" onClick={onClose} autoFocus data-cg="blocked-ok">
          {t('common.gotIt')}
        </Btn>
      }
    >
      <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
        {apps.map((a) => (
          <li key={a.key} style={{ padding: '6px 0', borderTop: '1px solid var(--cg-hair)' }}>
            <A to={appPath(a.key)} onClick={onClose}>
              {tx(a.name)}
            </A>{' '}
            <Mono className="cg-grey">{a.key}</Mono>
          </li>
        ))}
      </ul>
    </Modal>
  );
}

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
  const set = (k: keyof LocalUserInput, v: string) => setF((p) => ({ ...p, [k]: v }));
  return (
    <Modal
      title={t('catalog.createUser')}
      description={t('catalog.createUserHint')}
      onClose={onClose}
      width={560}
      dataCg="create-user"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} data-cg="create-user-submit">
            {t('common.create')}
          </Btn>
        </>
      }
    >
      <div className="cg-form">
        <Field label={t('catalog.username')} error={show('username')}>
          <Input value={f.username} onChange={(e) => set('username', e.target.value.trim())} className="cg-mono" invalid={!!show('username')} autoFocus data-cg="f-username" />
        </Field>
        <Field label={t('term.name')} error={show('displayName')}>
          <Input value={f.displayName} onChange={(e) => set('displayName', e.target.value)} invalid={!!show('displayName')} data-cg="f-name" />
        </Field>
        <Field label={t('catalog.email')} error={show('email')} optional>
          <Input value={f.email} onChange={(e) => set('email', e.target.value)} invalid={!!show('email')} data-cg="f-email" />
        </Field>
        <Field label={t('catalog.department')} optional>
          <Input value={f.dept} onChange={(e) => set('dept', e.target.value)} data-cg="f-dept" />
        </Field>
        <Field label={t('catalog.password')} error={show('password')} className="full">
          <Input type="password" value={f.password} onChange={(e) => set('password', e.target.value)} invalid={!!show('password')} autoComplete="new-password" data-cg="f-pw" />
        </Field>
      </div>
    </Modal>
  );
}

export function CreatePlatformDialog({ onClose }: { onClose: () => void }) {
  const { t } = useT();
  const s = useStore();
  const [type, setType] = useState<'GitLab' | 'GitHub'>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [tried, setTried] = useState(false);
  const u = url.trim().replace(/\/+$/, '');
  const errors = {
    url: !u ? t('catalog.err.url') : !/^https?:\/\/[^\s/]+(\/\S*)?$/.test(u) ? t('catalog.err.urlFormat') : s.platforms.some((p) => p.url === u) ? t('catalog.err.urlTaken') : undefined,
    token: !token.trim() ? t('catalog.err.token') : undefined,
  };
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.addPlatform({ id: `${type.toLowerCase()}-${s.platforms.length + 1}`, type, url: u, tokenTail: token.trim().slice(-4), addedAt: TODAY });
    onClose();
  };
  return (
    <Modal
      title={t('catalog.createPlatform')}
      description={t('catalog.createPlatformHint')}
      onClose={onClose}
      width={520}
      dataCg="create-platform"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind="primary" onClick={submit} data-cg="create-platform-submit">
            {t('catalog.register')}
          </Btn>
        </>
      }
    >
      <div className="cg-form" style={{ gridTemplateColumns: '1fr' }}>
        <Field label={t('catalog.platformType')}>
          <Select value={type} onChange={(e) => setType(e.target.value as 'GitLab' | 'GitHub')} data-cg="f-ptype">
            <option value="GitLab">GitLab</option>
            <option value="GitHub">GitHub</option>
          </Select>
        </Field>
        <Field label="URL" error={tried ? errors.url : undefined}>
          <Input value={url} onChange={(e) => setUrl(e.target.value)} className="cg-mono" placeholder="https://git.example.com" invalid={tried && !!errors.url} data-cg="f-url" />
        </Field>
        <Field label={t('catalog.serviceToken')} error={tried ? errors.token : undefined} hint={t('catalog.tokenHint')}>
          <Input type="password" value={token} onChange={(e) => setToken(e.target.value)} invalid={tried && !!errors.token} autoComplete="off" data-cg="f-token" />
        </Field>
      </div>
    </Modal>
  );
}
