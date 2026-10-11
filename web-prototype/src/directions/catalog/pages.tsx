import { useState } from 'react';
import { Plus } from 'lucide-react';
import { actions, can, soleOwnerOf, useStore, userName, type IdentityType, type User } from '../../data';
import { setLocale, useT, type Locale } from '../../i18n';
import { navigate } from '../../router';
import { BindIdentityDialog, CreatePlatformDialog, CreateUserDialog, SoleOwnerBlockedDialog } from './dialogs';
import { Shell } from './shell';
import { Btn, ConfirmDialog, Empty, Field, Input, Mono, Panel, Select, Status } from './ui';

// ---------------------------------------------------------------- login

export function Login() {
  const { t } = useT();
  const [type, setType] = useState<IdentityType>('ldap');
  const [user, setUser] = useState('elarsen');
  const [pw, setPw] = useState('demo-password');
  const [tried, setTried] = useState(false);
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    setTried(true);
    if (!user.trim() || !pw) return;
    navigate('/');
  };
  return (
    <div className="cg-login">
      <form className="cg-login-box" onSubmit={submit} noValidate data-cg="login">
        <h1 className="cg-h1" style={{ marginBottom: 16 }}>
          {t('catalog.brand')}
        </h1>
        <Panel title={t('catalog.signIn')}>
          <div className="cg-seg" role="radiogroup" aria-label={t('catalog.signInMethod')} style={{ justifySelf: 'start' }}>
            {(['ldap', 'local'] as const).map((k) => (
              <button key={k} type="button" role="radio" aria-checked={type === k} onClick={() => setType(k)} data-cg={`login-${k}`}>
                {t(`identity.${k}`)}
              </button>
            ))}
          </div>
          <Field label={type === 'ldap' ? t('catalog.ldapUsername') : t('catalog.localUsername')} error={tried && !user.trim() ? t('catalog.err.login') : undefined}>
            <Input value={user} onChange={(e) => setUser(e.target.value)} autoComplete="username" invalid={tried && !user.trim()} data-cg="login-user" />
          </Field>
          <Field label={t('catalog.password')} error={tried && !pw ? t('catalog.err.passwordRequired') : undefined}>
            <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="current-password" invalid={tried && !pw} data-cg="login-pw" />
          </Field>
          <Btn kind="primary" type="submit" data-cg="login-submit">
            {t('catalog.signIn')}
          </Btn>
        </Panel>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------- settings

export function Settings() {
  const { t, tx, tm, locale, fmtDate } = useT();
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState(false);
  const [unbinding, setUnbinding] = useState<IdentityType | null>(null);
  const free = (['ldap', 'local'] as IdentityType[]).filter((ty) => !me.identities.some((i) => i.type === ty));
  const last = me.identities.length <= 1;

  return (
    <Shell crumbs={[{ label: t('catalog.settings') }]}>
      <div className="cg-page-head">
        <h1 className="cg-h1">{t('catalog.settings')}</h1>
      </div>
      <div style={{ maxWidth: 860 }}>
        <Panel title={t('catalog.profile')} data-cg="profile">
          <dl className="cg-kv-rows">
            <dt>{t('term.name')}</dt>
            <dd>{tx(me.displayName)}</dd>
            <dt>{t('catalog.username')}</dt>
            <dd>
              <Mono>{me.username}</Mono>
            </dd>
            <dt>{t('catalog.email')}</dt>
            <dd>{me.email || '—'}</dd>
            <dt>{t('catalog.department')}</dt>
            <dd>{tx(me.dept) || '—'}</dd>
            <dt>{t('catalog.role')}</dt>
            <dd>{t(`role.${s.role}`)}</dd>
            <dt>{t('term.status')}</dt>
            <dd>
              <Status status={me.status} />
            </dd>
          </dl>
          <p className="cg-hint" style={{ marginTop: 12 }}>{t('catalog.profileReadOnly')}</p>
        </Panel>

        <Panel
          title={t('term.loginIdentities')}
          bodyClass={null}
          actions={
            <Btn sm onClick={() => setBinding(true)} disabled={free.length === 0} title={free.length === 0 ? t('catalog.allBound') : undefined} data-cg="bind-open">
              <Plus size={14} aria-hidden />
              {t('catalog.bindIdentity')}
            </Btn>
          }
          data-cg="identities"
        >
          <table className="cg-table">
            <thead>
              <tr>
                <th>{t('catalog.identityType')}</th>
                <th>{t('catalog.loginName')}</th>
                <th>{t('catalog.boundOn')}</th>
                <th>{t('catalog.boundBy')}</th>
                <th>
                  <span className="sr-only">{t('catalog.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {me.identities.map((i) => (
                <tr key={i.type} data-cg="identity-row" data-type={i.type}>
                  <td>{t(`identity.${i.type}`)}</td>
                  <td>
                    <Mono>{i.login}</Mono>
                  </td>
                  <td className="cg-nowrap">{fmtDate(i.boundAt)}</td>
                  <td>{tm(i.boundBy)}</td>
                  <td style={{ textAlign: 'right' }}>
                    <Btn sm onClick={() => setUnbinding(i.type)} disabled={last} title={last ? t('catalog.lastIdentity') : undefined} data-cg="unbind">
                      {t('catalog.unbind')}
                    </Btn>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {last && <p className="cg-hint" style={{ padding: '8px 16px 12px' }}>{t('catalog.lastIdentity')}</p>}
        </Panel>

        <Panel title={t('common.language')} data-cg="language">
          <Field label={t('common.language')} className="" hint={t('catalog.languageHint')}>
            <Select value={locale} onChange={(e) => setLocale(e.target.value as Locale)} style={{ width: 220 }} data-cg="settings-lang">
              <option value="en">{t('lang.en')}</option>
              <option value="zh">{t('lang.zh')}</option>
            </Select>
          </Field>
        </Panel>
      </div>

      {binding && <BindIdentityDialog user={me} onClose={() => setBinding(false)} />}
      {unbinding && (
        <ConfirmDialog
          danger
          title={t('catalog.unbindTitle', { type: t(`identity.${unbinding}`) })}
          description={t('catalog.unbindBody')}
          confirmLabel={t('catalog.unbind')}
          onClose={() => setUnbinding(null)}
          onConfirm={() => {
            actions.unbindIdentity(me.username, unbinding);
            setUnbinding(null);
          }}
        />
      )}
    </Shell>
  );
}

// ---------------------------------------------------------------- admin: code platforms

export function AdminPlatforms() {
  const { t, fmtDate } = useT();
  const s = useStore();
  const [creating, setCreating] = useState(false);
  return (
    <Shell crumbs={[{ label: t('catalog.administration') }, { label: t('term.codePlatforms') }]} adminOnly>
      <div className="cg-page-head">
        <h1 className="cg-h1">{t('term.codePlatforms')}</h1>
      </div>
      <Panel
        title={`${t('term.codePlatforms')} (${s.platforms.length})`}
        bodyClass={null}
        actions={
          <Btn kind="primary" sm onClick={() => setCreating(true)} data-cg="create-platform-open">
            <Plus size={14} aria-hidden />
            {t('catalog.createPlatform')}
          </Btn>
        }
        data-cg="platforms"
      >
        {s.platforms.length === 0 ? (
          <Empty title={t('catalog.noPlatforms')}>
            <p>{t('catalog.noPlatformsBody')}</p>
          </Empty>
        ) : (
          <table className="cg-table">
            <thead>
              <tr>
                <th>{t('catalog.platformType')}</th>
                <th>URL</th>
                <th>{t('catalog.serviceToken')}</th>
                <th>{t('term.applications')}</th>
                <th>{t('catalog.added')}</th>
              </tr>
            </thead>
            <tbody>
              {s.platforms.map((p) => (
                <tr key={p.id} data-cg="platform-row">
                  <td>{p.type}</td>
                  <td>
                    <Mono>{p.url}</Mono>
                  </td>
                  <td>
                    <Mono className="cg-grey">••••{p.tokenTail}</Mono>
                  </td>
                  <td>{s.apps.filter((a) => a.repo.platformId === p.id).length}</td>
                  <td className="cg-nowrap">{fmtDate(p.addedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Panel>
      {creating && <CreatePlatformDialog onClose={() => setCreating(false)} />}
    </Shell>
  );
}

// ---------------------------------------------------------------- admin: users

export function AdminUsers() {
  const { t, tx } = useT();
  const s = useStore();
  const [creating, setCreating] = useState(false);
  const [blocked, setBlocked] = useState<User | null>(null);
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<User | null>(null);
  const perms = can(s.role);

  const toggle = (u: User) => {
    if (u.status === 'Enabled') {
      if (soleOwnerOf(s, u.username).length) setBlocked(u);
      else setDisabling(u);
    } else actions.setUserStatus(u.username, 'Enabled');
  };

  return (
    <Shell crumbs={[{ label: t('catalog.administration') }, { label: t('term.users') }]} adminOnly>
      <div className="cg-page-head">
        <h1 className="cg-h1">{t('term.users')}</h1>
      </div>
      <Panel
        title={`${t('term.users')} (${s.users.length})`}
        bodyClass={null}
        actions={
          perms.admin && (
            <Btn kind="primary" sm onClick={() => setCreating(true)} data-cg="create-user-open">
              <Plus size={14} aria-hidden />
              {t('catalog.createUser')}
            </Btn>
          )
        }
        data-cg="users"
      >
        <table className="cg-table">
          <thead>
            <tr>
              <th>{t('term.name')}</th>
              <th>{t('catalog.username')}</th>
              <th>{t('catalog.email')}</th>
              <th>{t('catalog.department')}</th>
              <th>{t('term.loginIdentities')}</th>
              <th>{t('term.status')}</th>
              <th>
                <span className="sr-only">{t('catalog.actions')}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {s.users.map((u) => {
              const full = u.identities.length >= 2;
              return (
                <tr key={u.username} data-cg="user-row" data-user={u.username}>
                  <td className="cg-nowrap" style={{ fontWeight: 500 }}>{userName(u.username)}</td>
                  <td>
                    <Mono>{u.username}</Mono>
                  </td>
                  <td style={{ overflowWrap: 'anywhere' }}>{u.email || <span className="cg-grey">—</span>}</td>
                  <td>{tx(u.dept) || <span className="cg-grey">—</span>}</td>
                  <td>{u.identities.map((i) => t(`identity.${i.type}`)).join(' · ')}</td>
                  <td>
                    <Status status={u.status} />
                  </td>
                  <td>
                    <div className="cg-actions" style={{ justifyContent: 'flex-end' }}>
                      <Btn sm onClick={() => setBinding(u)} disabled={full} title={full ? t('catalog.allBound') : undefined} data-cg="user-bind">
                        {t('catalog.bindIdentity')}
                      </Btn>
                      <Btn sm onClick={() => toggle(u)} data-cg={u.status === 'Enabled' ? 'user-disable' : 'user-enable'}>
                        {u.status === 'Enabled' ? t('catalog.disable') : t('catalog.enable')}
                      </Btn>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Panel>
      {creating && <CreateUserDialog onClose={() => setCreating(false)} />}
      {blocked && <SoleOwnerBlockedDialog user={blocked} onClose={() => setBlocked(null)} />}
      {binding && <BindIdentityDialog user={binding} asAdmin onClose={() => setBinding(null)} />}
      {disabling && (
        <ConfirmDialog
          title={t('catalog.disableTitle', { name: userName(disabling.username) })}
          description={t('catalog.disableBody')}
          confirmLabel={t('catalog.disable')}
          onClose={() => setDisabling(null)}
          onConfirm={() => {
            actions.setUserStatus(disabling.username, 'Disabled');
            setDisabling(null);
          }}
        />
      )}
    </Shell>
  );
}
