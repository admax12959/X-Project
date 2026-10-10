import { useState } from 'react';
import { actions, TODAY, useStore, type IdentityType, type User } from '../data';
import { Shell } from '../Shell';
import { Button, Dialog, DialogActions, Field, Input, Mono, PageHeader, SectionTitle } from '../ui';

export const IDENTITY_LABEL: Record<IdentityType, string> = { ldap: 'LDAP', local: '本地账号密码' };

export function Settings() {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState<IdentityType | null>(null);
  const hasLdap = me.identities.some((i) => i.type === 'ldap');
  const unbound = (['ldap', 'local'] as const).filter((t) => !me.identities.some((i) => i.type === t));

  return (
    <Shell title="个人设置">
      <PageHeader title="个人设置" />
      <div className="grid max-w-4xl grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-14">
        <section>
          <SectionTitle>用户信息</SectionTitle>
          {hasLdap && <p className="mb-4 text-[13px] text-ink-2">已绑定 LDAP，显示名、邮箱、部门以 LDAP 为准，每次登录时同步，这里不能修改。</p>}
          <div className="space-y-4">
            <Field label="用户名"><Input readOnly value={me.username} /></Field>
            <Field label="显示名"><Input readOnly={hasLdap} defaultValue={me.displayName} /></Field>
            <Field label="邮箱"><Input readOnly={hasLdap} defaultValue={me.email} /></Field>
            <Field label="部门"><Input readOnly={hasLdap} defaultValue={me.dept ?? ''} /></Field>
            <Field label="手机号（可选）"><Input defaultValue={me.phone ?? ''} /></Field>
            <Button kind="primary">保存</Button>
          </div>
        </section>

        <section>
          <SectionTitle>登录身份</SectionTitle>
          <p className="mb-4 text-[13px] text-ink-2">同一个用户可以用多种登录身份登录。工具不会按邮箱自动合并用户，追加身份需要在这里用新身份完成一次认证。</p>
          <ul className="divide-y divide-line-2 border-y border-line">
            {me.identities.map((i) => (
              <li key={i.type} className="flex items-baseline gap-4 py-3">
                <span className="w-28 font-medium">{IDENTITY_LABEL[i.type]}</span>
                <span className="flex-1">
                  <Mono>{i.login}</Mono>
                  <div className="text-[12px] text-ink-3">{i.boundAt} · {i.boundBy}</div>
                </span>
                <Button
                  disabled={me.identities.length === 1}
                  title={me.identities.length === 1 ? '至少保留一个登录身份' : undefined}
                  onClick={() => confirm(`解绑后不能再用${IDENTITY_LABEL[i.type]}登录。确定？`) && actions.unbindIdentity(me.username, i.type)}
                >
                  解绑
                </Button>
              </li>
            ))}
          </ul>
          {me.identities.length === 1 && <p className="mt-2 text-[12px] text-ink-3">只剩一个登录身份时不能解绑。</p>}
          {unbound.length > 0 && (
            <div className="mt-4 flex gap-2">
              {unbound.map((t) => (
                <Button key={t} onClick={() => setBinding(t)}>绑定{IDENTITY_LABEL[t]}</Button>
              ))}
            </div>
          )}
        </section>
      </div>
      {binding && <BindDialog user={me} type={binding} by="本人绑定" onClose={() => setBinding(null)} />}
    </Shell>
  );
}

export function BindDialog({ user, type, by, onClose }: { user: User; type: IdentityType; by: string; onClose: () => void }) {
  const [login, setLogin] = useState(type === 'local' ? user.username : '');
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const ldap = type === 'ldap';
  const ok = login.trim() && pw && (ldap || pw === pw2);
  return (
    <Dialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={`为 ${user.displayName} 绑定${IDENTITY_LABEL[type]}`}
      description={ldap ? '用要绑定的 LDAP 身份完成一次认证，证明归属。绑定后显示名、邮箱、部门改为从 LDAP 同步。' : '设置本地登录用的用户名和密码，之后可以不经过 LDAP 登录。'}
    >
      <div className="space-y-4">
        <Field label={ldap ? 'LDAP 用户名' : '本地登录名'}>
          <Input value={login} onChange={(e) => setLogin(e.target.value)} autoFocus />
        </Field>
        <Field label={ldap ? 'LDAP 密码' : '密码'}>
          <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} />
        </Field>
        {!ldap && (
          <Field label="再输一次密码" error={pw2 && pw !== pw2 ? '两次输入不一致' : undefined}>
            <Input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} />
          </Field>
        )}
      </div>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind="primary" disabled={!ok} onClick={() => (actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy: by }), onClose())}>
          {ldap ? '认证并绑定' : '绑定'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
