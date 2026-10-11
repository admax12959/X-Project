import { Plus, Search } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { actions, can, useStore, type IdentityType, type User } from '../../data';
import { setLocale, useT, type Locale } from '../../i18n';
import { navigate } from '../../router';
import { BindDialog, CreateUserDialog, DisableUserDialog, PlatformDialog } from './dialogs';
import { Shell } from './shell';
import { showing } from './list';
import { Btn, Container, Dialog, Empty, Field, Input, KV, LinkBtn, Mono, Select, UserStatusText } from './ui';

// ---------------------------------------------------------------- Login

export function Login() {
  const { t } = useT();
  const [mode, setMode] = useState<'ldap' | 'local'>('ldap');
  const [user, setUser] = useState('');
  const [pw, setPw] = useState('');
  const [tried, setTried] = useState(false);
  const submit = () => {
    setTried(true);
    if (!user.trim() || !pw) return;
    navigate('/');
  };
  return (
    <div className="cs-login">
      <div className="cs-brand" style={{ marginBottom: 16, fontSize: 18 }}>
        {t('console.brand')}
      </div>
      <form
        className="cs-login-card"
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
        noValidate
      >
        <h1 className="cs-h2" style={{ fontSize: 20, marginBottom: 16 }}>
          {t('console.login.title')}
        </h1>
        <div role="tablist" aria-label={t('console.login.title')} className="cs-seg" style={{ marginBottom: 16 }}>
          {(['ldap', 'local'] as const).map((m) => (
            <button key={m} type="button" role="tab" aria-selected={mode === m} onClick={() => setMode(m)}>
              {t(`identity.${m}`)}
            </button>
          ))}
        </div>
        <div style={{ display: 'grid', gap: 14 }}>
          <Field label={t(mode === 'ldap' ? 'console.bind.ldapUser' : 'console.bind.localUser')} error={tried && !user.trim() ? t('console.login.err.user') : undefined}>
            <Input value={user} onChange={(e) => setUser(e.target.value)} invalid={tried && !user.trim()} autoFocus />
          </Field>
          <Field label={t('console.bind.password')} error={tried && !pw ? t('console.bind.err.password') : undefined}>
            <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} invalid={tried && !pw} />
          </Field>
          <Btn kind="primary" type="submit" style={{ justifyContent: 'center' }}>
            {t('console.login.submit')}
          </Btn>
        </div>
      </form>
    </div>
  );
}

// ---------------------------------------------------------------- Settings

export function Settings() {
  const { t, tx, tm, locale, fmtDate } = useT();
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState<IdentityType | null>(null);
  const [unbinding, setUnbinding] = useState<IdentityType | null>(null);
  const missing = (['ldap', 'local'] as const).filter((x) => !me.identities.some((i) => i.type === x));
  const only = me.identities.length <= 1;
  return (
    <Shell>
      <div className="cs-titlebar">
        <h1 className="cs-h1">{t('console.settings')}</h1>
      </div>
      <Container title={t('console.profile')}>
        <KV
          rows={[
            { label: t('term.name'), value: tx(me.displayName) },
            { label: t('console.username'), value: <Mono>{me.username}</Mono> },
            { label: t('console.email'), value: me.email || '—' },
            { label: t('console.dept'), value: me.dept ? tx(me.dept) : '—' },
            { label: t('console.role'), value: t(`role.${s.role}`) },
            { label: t('term.status'), value: <UserStatusText status={me.status} /> },
          ]}
        />
      </Container>
      <Container
        flush
        title={`${t('term.loginIdentities')} (${me.identities.length})`}
        actions={missing.map((x) => (
          <Btn key={x} onClick={() => setBinding(x)}>
            <Plus size={16} aria-hidden /> {t('console.bind.button', { type: t(`identity.${x}`) })}
          </Btn>
        ))}
      >
        <div style={{ overflowX: 'auto', borderTop: '1px solid var(--c-line)' }}>
          <table className="cs-table">
            <thead>
              <tr>
                <th>{t('console.identityType')}</th>
                <th>{t('console.loginName')}</th>
                <th>{t('console.boundOn')}</th>
                <th>{t('console.boundBy')}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {me.identities.map((i) => (
                <tr key={i.type}>
                  <td className="nw">{t(`identity.${i.type}`)}</td>
                  <td className="nw">
                    <Mono>{i.login}</Mono>
                  </td>
                  <td className="nw">{fmtDate(i.boundAt)}</td>
                  <td className="nw">{tm(i.boundBy)}</td>
                  <td className="nw" style={{ textAlign: 'right' }}>
                    <LinkBtn disabled={only} title={only ? t('console.unbind.last') : undefined} onClick={() => setUnbinding(i.type)}>
                      {t('console.unbind')}
                    </LinkBtn>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Container>
      <Container title={t('common.language')}>
        <div style={{ maxWidth: 280 }}>
          <Select aria-label={t('common.language')} value={locale} onChange={(e) => setLocale(e.target.value as Locale)}>
            <option value="en">{t('lang.en')}</option>
            <option value="zh">{t('lang.zh')}</option>
          </Select>
          <span className="cs-hint">{t('console.language.hint')}</span>
        </div>
      </Container>
      {binding && <BindDialog user={me} type={binding} byAdmin={false} onClose={() => setBinding(null)} />}
      {unbinding && (
        <Dialog
          onClose={() => setUnbinding(null)}
          title={t('console.unbind.title', { type: t(`identity.${unbinding}`) })}
          description={t('console.unbind.desc')}
          footer={
            <>
              <Btn onClick={() => setUnbinding(null)}>{t('common.cancel')}</Btn>
              <Btn
                kind="danger"
                onClick={() => {
                  actions.unbindIdentity(me.username, unbinding);
                  setUnbinding(null);
                }}
              >
                {t('console.unbind')}
              </Btn>
            </>
          }
        />
      )}
    </Shell>
  );
}

// ---------------------------------------------------------------- Admin

function AdminOnly({ children }: { children: ReactNode }) {
  const { t } = useT();
  const s = useStore();
  if (can(s.role).admin) return <>{children}</>;
  return (
    <Shell>
      <div className="cs-container">
        <Empty title={t('console.adminOnly')}>
          <LinkBtn onClick={() => navigate('/')}>{t('console.backToList')}</LinkBtn>
        </Empty>
      </div>
    </Shell>
  );
}

export function AdminPlatforms() {
  const { t, fmtDate } = useT();
  const s = useStore();
  const [adding, setAdding] = useState(false);
  return (
    <AdminOnly>
      <Shell>
        <div className="cs-titlebar">
          <h1 className="cs-h1">
            {t('term.codePlatforms')} <span className="cs-count">({s.platforms.length})</span>
          </h1>
          <div className="cs-actions">
            <Btn kind="primary" onClick={() => setAdding(true)}>
              <Plus size={16} aria-hidden /> {t('console.platform.title')}
            </Btn>
          </div>
        </div>
        <div>
          <div className="cs-showing" role="status" style={{ marginTop: 0 }}>
            {showing(t, s.platforms.length, s.platforms.length)}
          </div>
          <div className="cs-tablewrap">
            <table className="cs-table">
              <thead>
                <tr>
                  <th>{t('console.platform.type')}</th>
                  <th style={{ minWidth: 260 }}>{t('console.platform.url')}</th>
                  <th>{t('console.platform.token')}</th>
                  <th>{t('console.added')}</th>
                  <th className="num">{t('term.applications')}</th>
                </tr>
              </thead>
              <tbody>
                {s.platforms.map((p) => (
                  <tr key={p.id}>
                    <td className="nw">{p.type}</td>
                    <td className="nw">
                      <Mono>{p.url}</Mono>
                    </td>
                    <td className="nw">
                      <Mono>••••{p.tokenTail}</Mono>
                    </td>
                    <td className="nw">{fmtDate(p.addedAt)}</td>
                    <td className="num">{s.apps.filter((a) => a.repo.platformId === p.id).length}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {s.platforms.length === 0 && <Empty title={t('console.platform.empty')} />}
          </div>
        </div>
        {adding && <PlatformDialog onClose={() => setAdding(false)} />}
      </Shell>
    </AdminOnly>
  );
}

export function AdminUsers() {
  const { t, tx } = useT();
  const s = useStore();
  const [q, setQ] = useState('');
  const [creating, setCreating] = useState(false);
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<{ user: User; type: IdentityType } | null>(null);
  const needle = q.trim().toLowerCase();
  const rows = s.users.filter((u) => !needle || [u.username, tx(u.displayName), u.email, u.dept ? tx(u.dept) : ''].some((x) => x.toLowerCase().includes(needle)));
  return (
    <AdminOnly>
      <Shell>
        <div className="cs-titlebar">
          <h1 className="cs-h1">
            {t('term.users')} <span className="cs-count">({s.users.length})</span>
          </h1>
          <div className="cs-actions">
            <Btn kind="primary" onClick={() => setCreating(true)}>
              <Plus size={16} aria-hidden /> {t('console.createUser.title')}
            </Btn>
          </div>
        </div>
        <div>
          <div className="cs-filters">
            <div className="cs-search">
              <Search size={16} aria-hidden />
              <Input aria-label={t('common.search')} placeholder={t('console.users.search')} value={q} onChange={(e) => setQ(e.target.value)} />
            </div>
            {q && (
              <LinkBtn className="cs-small" onClick={() => setQ('')}>
                {t('console.filter.clear')}
              </LinkBtn>
            )}
          </div>
          <div className="cs-showing" role="status">
            {showing(t, rows.length, s.users.length)}
          </div>
          <div className="cs-tablewrap">
            <table className="cs-table">
              <thead>
                <tr>
                  <th>{t('term.name')}</th>
                  <th>{t('console.username')}</th>
                  <th>{t('console.email')}</th>
                  <th>{t('console.dept')}</th>
                  <th>{t('term.loginIdentities')}</th>
                  <th>{t('term.status')}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {rows.map((u) => {
                  const missing = (['ldap', 'local'] as const).filter((x) => !u.identities.some((i) => i.type === x));
                  return (
                    <tr key={u.username}>
                      <td className="nw">{tx(u.displayName)}</td>
                      <td className="nw">
                        <Mono>{u.username}</Mono>
                      </td>
                      <td style={{ maxWidth: 200 }}>
                        <span className="cs-clip" title={u.email}>
                          {u.email || '—'}
                        </span>
                      </td>
                      <td className="nw">{u.dept ? tx(u.dept) : '—'}</td>
                      <td className="nw">{u.identities.map((i) => t(`identity.${i.type}`)).join(', ')}</td>
                      <td>
                        <UserStatusText status={u.status} />
                      </td>
                      <td className="nw" style={{ textAlign: 'right' }}>
                        <span style={{ display: 'inline-flex', gap: 16 }}>
                          {missing.map((x) => (
                            <LinkBtn key={x} aria-label={`${t('console.bind.button', { type: t(`identity.${x}`) })}: ${u.username}`} onClick={() => setBinding({ user: u, type: x })}>
                              {t('console.bind.button', { type: t(`identity.${x}`) })}
                            </LinkBtn>
                          ))}
                          {u.status === 'Enabled' ? (
                            <LinkBtn aria-label={`${t('console.disable.submit')}: ${u.username}`} onClick={() => setDisabling(u)}>
                              {t('console.disable.submit')}
                            </LinkBtn>
                          ) : (
                            <LinkBtn aria-label={`${t('console.enable')}: ${u.username}`} onClick={() => actions.setUserStatus(u.username, 'Enabled')}>
                              {t('console.enable')}
                            </LinkBtn>
                          )}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {rows.length === 0 && <Empty title={t('console.users.noMatch')} />}
          </div>
        </div>
        {creating && <CreateUserDialog onClose={() => setCreating(false)} />}
        {disabling && <DisableUserDialog user={disabling} onClose={() => setDisabling(null)} />}
        {binding && <BindDialog user={binding.user} type={binding.type} byAdmin onClose={() => setBinding(null)} />}
      </Shell>
    </AdminOnly>
  );
}
