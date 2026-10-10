import { useState } from 'react';
import { actions, TODAY, useStore, type IdentityType, type User } from '../../data';
import { Crumbs, PageHead, SectionHead, Shell } from './Shell';
import { Button, Code, Dialog, DialogActions, Field, Input } from './ui';

export const IDENTITY_LABEL: Record<IdentityType, string> = { ldap: 'LDAP', local: '本地账号密码' };

export function Settings() {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState<IdentityType | null>(null);
  const [unbinding, setUnbinding] = useState<IdentityType | null>(null);
  const hasLdap = me.identities.some((i) => i.type === 'ldap');
  const unbound = (['ldap', 'local'] as const).filter((t) => !me.identities.some((i) => i.type === t));
  const last = me.identities.length === 1;

  return (
    <Shell crumbs={<Crumbs items={[{ label: '个人设置' }]} />}>
      <PageHead title="个人设置" sub={<>你在工具里只有一份用户信息，可以用下面任何一个登录身份登录。</>} />
      <div className="grid grid-cols-12 gap-x-10 gap-y-12">
        <section className="col-span-7 max-[1100px]:col-span-12" aria-labelledby="ids">
          <SectionHead id="ids">登录身份</SectionHead>
          <ul className="border-b border-[var(--lc-ink)]">
            {me.identities.map((i) => (
              <li key={i.type} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 border-b border-[var(--lc-rule)] py-5 last:border-b-0">
                <div className="min-w-0">
                  <div className="text-[length:var(--lc-text-lg)] leading-tight font-bold">{IDENTITY_LABEL[i.type]}</div>
                  <div className="mt-1 text-[var(--lc-ink-2)]">
                    登录名 <Code className="font-semibold text-[var(--lc-ink)]">{i.login}</Code>
                    <span className="lc-num ml-4 text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                      {i.boundAt} 绑定 · {i.boundBy}
                    </span>
                  </div>
                </div>
                <Button disabled={last} aria-describedby={last ? 'last-id' : undefined} onClick={() => setUnbinding(i.type)}>
                  解绑
                </Button>
              </li>
            ))}
          </ul>
          {last && (
            <p id="last-id" className="mt-3 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
              只剩一个登录身份，不能解绑。先绑定另一个，再解绑这个。
            </p>
          )}
          {unbound.length > 0 && (
            <div className="mt-6">
              <p className="mb-3 max-w-[60ch] text-[var(--lc-ink-2)]">追加登录身份需要用新身份完成一次认证。工具不会按邮箱自动合并用户。</p>
              <div className="flex gap-2">
                {unbound.map((t) => (
                  <Button key={t} onClick={() => setBinding(t)}>
                    绑定{IDENTITY_LABEL[t]}
                  </Button>
                ))}
              </div>
            </div>
          )}
        </section>

        <section className="col-span-5 max-[1100px]:col-span-12" aria-labelledby="info">
          <SectionHead id="info">用户信息</SectionHead>
          <p className="mb-4 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
            {hasLdap ? '已绑定 LDAP：显示名、邮箱、部门以 LDAP 为准，每次登录时同步，这里不能修改。' : '没有绑定 LDAP。这些信息由管理员创建用户时填写，这里只能查看。'}
          </p>
          <dl className="grid grid-cols-[72px_minmax(0,1fr)] gap-x-4 gap-y-3">
            <dt className="lc-muted">用户名</dt>
            <dd><Code>{me.username}</Code></dd>
            <dt className="lc-muted">显示名</dt>
            <dd>{me.displayName}</dd>
            <dt className="lc-muted">邮箱</dt>
            <dd className="lc-code">{me.email || '—'}</dd>
            <dt className="lc-muted">部门</dt>
            <dd>{me.dept ?? '—'}</dd>
            {me.phone && (
              <>
                <dt className="lc-muted">手机号</dt>
                <dd className="lc-num">{me.phone}</dd>
              </>
            )}
          </dl>
        </section>
      </div>
      {binding && <BindDialog user={me} type={binding} by="本人绑定" self onClose={() => setBinding(null)} />}
      {unbinding && (
        <Dialog onClose={() => setUnbinding(null)} title={`解绑${IDENTITY_LABEL[unbinding]}？`} description={`解绑后不能再用${IDENTITY_LABEL[unbinding]}登录。${unbinding === 'ldap' ? '显示名、邮箱、部门停止从 LDAP 同步，保留当前值。' : ''}之后可以重新绑定。`}>
          <DialogActions>
            <Button onClick={() => setUnbinding(null)}>取消</Button>
            <Button kind="danger" autoFocus onClick={() => (actions.unbindIdentity(me.username, unbinding), setUnbinding(null))}>
              解绑
            </Button>
          </DialogActions>
        </Dialog>
      )}
    </Shell>
  );
}

/**
 * Binding proves ownership of the new identity.
 * Self: authenticate with it (LDAP password, or set the local password).
 * Admin: names the LDAP account, or sets an initial local password.
 */
export function BindDialog({ user, type, by, self = false, onClose }: { user: User; type: IdentityType; by: string; self?: boolean; onClose: () => void }) {
  const s = useStore();
  const ldap = type === 'ldap';
  const [login, setLogin] = useState(ldap ? '' : user.username);
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [tried, setTried] = useState(false);
  const owner = s.users.find((u) => u.identities.some((i) => i.type === type && i.login === login.trim()));
  const needPw = !ldap || self;
  const errors = {
    login: !login.trim() ? '填写登录名' : owner ? `这个${IDENTITY_LABEL[type]}已绑定到用户 ${owner.displayName}` : undefined,
    pw: needPw && !pw ? (ldap ? '填写 LDAP 密码完成认证' : '设置密码') : !ldap && pw.length < 8 ? '密码至少 8 位' : undefined,
    pw2: !ldap && pw !== pw2 ? '两次输入不一致' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);
  const submit = () => {
    setTried(true);
    if (!ok) return;
    actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy: by });
    onClose();
  };
  const desc = ldap
    ? self
      ? '用要绑定的 LDAP 身份认证一次，证明它属于你。绑定后显示名、邮箱、部门改为从 LDAP 同步。'
      : `为 ${user.displayName} 绑定 LDAP 身份。绑定后显示名、邮箱、部门改为从 LDAP 同步。`
    : self
      ? '设置本地登录用的登录名和密码，之后可以不经过 LDAP 登录。'
      : `为 ${user.displayName} 设置本地登录名和初始密码，由你线下告知本人。`;
  return (
    <Dialog onClose={onClose} title={`${self ? '' : `为 ${user.displayName} `}绑定${IDENTITY_LABEL[type]}`} description={desc}>
      <form onSubmit={(e) => (e.preventDefault(), submit())} noValidate>
        <Field label={ldap ? 'LDAP 用户名' : '本地登录名'} error={tried ? errors.login : undefined}>
          <Input value={login} onChange={(e) => setLogin(e.target.value)} className="lc-code" autoFocus />
        </Field>
        {needPw && (
          <Field className="mt-2" label={ldap ? 'LDAP 密码' : '密码'} error={tried ? errors.pw : undefined} help={ldap ? '只用于这次认证，工具不保存' : '至少 8 位'}>
            <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} />
          </Field>
        )}
        {!ldap && (
          <Field className="mt-2" label="再输一次密码" error={(tried || pw2) && errors.pw2 ? errors.pw2 : undefined}>
            <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} />
          </Field>
        )}
        <DialogActions>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit">
            {ldap && self ? '认证并绑定' : '绑定'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
