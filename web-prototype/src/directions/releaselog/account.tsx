import { Tabs } from '@base-ui/react/tabs';
import { useState, type ReactNode } from 'react';
import { actions, can, soleOwnerOf, TODAY, useStore, validateLocalUser, type CodePlatform, type IdentityType, type LocalUserInput, type User } from '../../data';
import { navigate } from '../../router';
import { Shell } from './Shell';
import { Button, Dialog, Field, Input, LinkButton, Mono, Segmented } from './ui';

export const IDENTITY_LABEL: Record<IdentityType, string> = { ldap: 'LDAP', local: '本地账号密码' };

/* ---------------- 登录 ---------------- */

export function Login() {
  return (
    <div className="grid min-h-screen grid-cols-1 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
      <div className="flex flex-col justify-between px-10 py-10 lg:px-16" style={{ background: 'var(--rl-paper-2)', borderRight: '1px solid var(--rl-rule)' }}>
        <div className="rl-t-md font-semibold">应用维护工具</div>
        <div className="my-12">
          <h1 className="rl-t-xl rl-prose">登记应用、维护线和每一个版本</h1>
          <p className="mt-3 rl-prose rl-ink-2">谁是 Owner 和 PE，代码在哪个仓库，哪条维护线从哪个版本分叉、什么时候终止，都记在这里。</p>
        </div>
        <div className="rl-ink-3">内部工具 · 登录有问题请联系管理员</div>
      </div>
      <div className="flex items-center px-10 py-12 lg:px-20">
        <div className="w-full max-w-[400px]">
          <h2 className="rl-t-lg">登录</h2>
          <Tabs.Root defaultValue="ldap" className="mt-5">
            <Tabs.List className="flex gap-6" style={{ borderBottom: '1px solid var(--rl-rule)' }}>
              {(['ldap', 'local'] as const).map((t) => (
                <Tabs.Tab key={t} value={t} className="rl-tab">
                  {IDENTITY_LABEL[t]}
                </Tabs.Tab>
              ))}
            </Tabs.List>
            <Tabs.Panel value="ldap" className="pt-6">
              <LoginForm kind="ldap" />
            </Tabs.Panel>
            <Tabs.Panel value="local" className="pt-6">
              <LoginForm kind="local" />
            </Tabs.Panel>
          </Tabs.Root>
        </div>
      </div>
    </div>
  );
}

function LoginForm({ kind }: { kind: IdentityType }) {
  const [u, setU] = useState(kind === 'ldap' ? 'linqiao' : '');
  const [p, setP] = useState(kind === 'ldap' ? 'prototype' : '');
  const [tried, setTried] = useState(false);
  const errU = tried && !u.trim() ? '填写用户名' : undefined;
  const errP = tried && !p ? '填写密码' : undefined;
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        setTried(true);
        if (u.trim() && p) navigate('/apps');
      }}
    >
      <Field label={kind === 'ldap' ? 'LDAP 用户名' : '用户名'} error={errU} hint={kind === 'ldap' ? '首次登录自动创建用户，显示名、邮箱、部门从 LDAP 同步' : '本地账号密码由管理员创建，或在个人设置里自行绑定'}>
        {(f) => <Input f={f} value={u} onChange={(e) => setU(e.target.value)} autoComplete="username" />}
      </Field>
      <Field label="密码" error={errP}>{(f) => <Input f={f} type="password" value={p} onChange={(e) => setP(e.target.value)} autoComplete="current-password" />}</Field>
      <Button kind="primary" type="submit" className="mt-1 w-full">登录</Button>
    </form>
  );
}

/* ---------------- 个人设置 ---------------- */

export function Settings() {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState<IdentityType | null>(null);
  const [unbinding, setUnbinding] = useState<IdentityType | null>(null);
  const ldap = me.identities.some((i) => i.type === 'ldap');
  const unbound = (['ldap', 'local'] as const).filter((t) => !me.identities.some((i) => i.type === t));
  const last = me.identities.length === 1;

  return (
    <Shell crumbs={[{ label: '个人设置' }]}>
      <div className="px-8 pt-8">
        <h1 className="rl-t-xl">个人设置</h1>
        <div className="mt-8 grid grid-cols-1 gap-x-16 gap-y-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
          <section aria-labelledby="me-h">
            <h2 id="me-h" className="rl-t-lg">用户信息</h2>
            <p className="mt-1 rl-prose rl-ink-2">{ldap ? '已绑定 LDAP。显示名、邮箱、部门以 LDAP 为准，每次登录时同步，这里只读。' : '本地用户的信息由管理员维护。'}</p>
            <dl className="mt-4 grid grid-cols-[96px_minmax(0,1fr)]" style={{ borderTop: '1px solid var(--rl-ink)' }}>
              {(
                [
                  ['用户名', me.username],
                  ['显示名', me.displayName],
                  ['邮箱', me.email || <span className="rl-ink-3">未填写</span>],
                  ['部门', me.dept ?? <span className="rl-ink-3">未填写</span>],
                  ['手机号', me.phone ?? <span className="rl-ink-3">未填写</span>],
                ] as [string, ReactNode][]
              ).map(([k, v]) => (
                <div key={k} className="contents">
                  <dt className="py-3 rl-ink-3" style={{ borderBottom: '1px solid var(--rl-rule)' }}>{k}</dt>
                  <dd className="py-3 rl-t-md" style={{ borderBottom: '1px solid var(--rl-rule)' }}>{v}</dd>
                </div>
              ))}
            </dl>
          </section>

          <section aria-labelledby="id-h">
            <h2 id="id-h" className="rl-t-lg">登录身份</h2>
            <p className="mt-1 rl-prose rl-ink-2">同一个用户可以用多种登录身份登录。不会按邮箱自动合并，追加身份要用这个身份完成一次认证。</p>
            <ul className="mt-4" style={{ borderTop: '1px solid var(--rl-ink)' }}>
              {me.identities.map((i) => (
                <li key={i.type} className="grid grid-cols-[136px_minmax(0,1fr)_auto] items-center gap-4 py-3" style={{ borderBottom: '1px solid var(--rl-rule)' }}>
                  <span className="rl-t-md font-semibold">{IDENTITY_LABEL[i.type]}</span>
                  <span className="min-w-0">
                    <span className="rl-ink">{i.login}</span>
                    <span className="block rl-ink-3">{i.boundAt} 绑定 · {i.boundBy}</span>
                  </span>
                  <Button size="sm" disabled={last} onClick={() => setUnbinding(i.type)} aria-describedby={last ? 'last-id' : undefined}>解绑</Button>
                </li>
              ))}
            </ul>
            {last && <p id="last-id" className="mt-2 rl-ink-2">只剩一个登录身份，不能解绑。先绑定另一个，再解绑这个。</p>}
            {unbound.length > 0 && (
              <div className="mt-4 flex gap-2">
                {unbound.map((t) => (
                  <Button key={t} onClick={() => setBinding(t)}>绑定{IDENTITY_LABEL[t]}</Button>
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
      {binding && <BindDialog user={me} type={binding} by="本人绑定" onClose={() => setBinding(null)} />}
      {unbinding && (
        <Dialog
          onClose={() => setUnbinding(null)}
          title={`解绑${IDENTITY_LABEL[unbinding]}？`}
          description={`解绑后不能再用${IDENTITY_LABEL[unbinding]}登录，用户信息和历史记录不受影响。`}
          footer={
            <>
              <Button onClick={() => setUnbinding(null)}>取消</Button>
              <Button kind="danger" onClick={() => (actions.unbindIdentity(me.username, unbinding), setUnbinding(null))}>解绑</Button>
            </>
          }
        />
      )}
    </Shell>
  );
}

export function BindDialog({ user, type, by, onClose }: { user: User; type: IdentityType; by: string; onClose: () => void }) {
  const ldap = type === 'ldap';
  const [login, setLogin] = useState(ldap ? '' : user.username);
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [tried, setTried] = useState(false);
  const errors = {
    login: !login.trim() ? (ldap ? '填写 LDAP 用户名' : '填写本地登录名') : undefined,
    pw: !pw ? '填写密码' : !ldap && pw.length < 8 ? '至少 8 位' : undefined,
    pw2: !ldap && pw !== pw2 ? '两次输入不一致' : undefined,
  };
  const submit = () => {
    setTried(true);
    if (Object.values(errors).some(Boolean)) return;
    actions.bindIdentity(user.username, { type, login: login.trim(), boundAt: TODAY, boundBy: by });
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      title={`为 ${user.displayName} 绑定${IDENTITY_LABEL[type]}`}
      description={ldap ? '用要绑定的 LDAP 身份完成一次认证，证明这个身份属于该用户。绑定后显示名、邮箱、部门改为从 LDAP 同步。' : '设置本地登录名和密码，之后可以不经过 LDAP 登录。'}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>{ldap ? '认证并绑定' : '绑定'}</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())}>
        <Field label={ldap ? 'LDAP 用户名' : '本地登录名'} error={tried ? errors.login : undefined}>{(f) => <Input f={f} value={login} onChange={(e) => setLogin(e.target.value)} autoFocus />}</Field>
        <Field label={ldap ? 'LDAP 密码' : '密码'} error={tried ? errors.pw : undefined}>{(f) => <Input f={f} type="password" value={pw} onChange={(e) => setPw(e.target.value)} />}</Field>
        {!ldap && <Field label="再输一次密码" error={tried || pw2 ? errors.pw2 : undefined}>{(f) => <Input f={f} type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} />}</Field>}
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

/* ---------------- 管理后台 ---------------- */

function AdminOnly({ title, children }: { title: string; children: ReactNode }) {
  const s = useStore();
  if (!can(s.role).admin)
    return (
      <Shell crumbs={[{ label: '管理后台' }, { label: title }]}>
        <div className="px-8 pt-10">
          <h1 className="rl-t-xl">{title}</h1>
          <p className="mt-2 rl-ink-2">只有管理员可以管理{title}。</p>
        </div>
      </Shell>
    );
  return <Shell crumbs={[{ label: '管理后台' }, { label: title }]}>{children}</Shell>;
}

function PageHead({ title, sub, action }: { title: string; sub: string; action: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4 px-8 pt-8 pb-5">
      <div>
        <h1 className="rl-t-xl">{title}</h1>
        <p className="mt-1 rl-prose rl-ink-2">{sub}</p>
      </div>
      {action}
    </div>
  );
}

const TH = 'py-3 pr-4 text-left align-bottom rl-label';

export function AdminPlatforms() {
  const s = useStore();
  const [adding, setAdding] = useState(false);
  return (
    <AdminOnly title="代码平台">
      <PageHead title="代码平台" sub="应用建立仓库关联时从这里选择代码平台。服务令牌只需要只读权限，保存后只显示末四位。" action={<Button kind="primary" onClick={() => setAdding(true)}>登记代码平台</Button>} />
      <div className="px-8">
        <table className="w-full border-collapse">
          <thead style={{ borderBottom: '1px solid var(--rl-ink)' }}>
            <tr>
              <th className={TH}>代码平台</th>
              <th className={TH}>只读服务令牌</th>
              <th className={TH}>仓库关联</th>
              <th className={TH}>登记时间</th>
            </tr>
          </thead>
          <tbody>
            {s.platforms.map((p) => {
              const used = s.apps.filter((a) => a.repo.platformId === p.id);
              return (
                <tr key={p.id} style={{ borderBottom: '1px solid var(--rl-rule)' }}>
                  <td className="py-4 pr-4">
                    <span className="rl-t-md font-semibold">{p.type}</span>
                    <Mono className="ml-3 rl-ink-2">{p.url}</Mono>
                  </td>
                  <td className="py-4 pr-4"><Mono className="rl-ink-2">••••••••{p.tokenTail}</Mono></td>
                  <td className="py-4 pr-4">{used.length ? used.map((a) => a.name).join('、') : <span className="rl-ink-3">还没有应用使用</span>}</td>
                  <td className="py-4 pr-4 rl-ink-2">{p.addedAt}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {adding && <PlatformDialog onClose={() => setAdding(false)} />}
    </AdminOnly>
  );
}

function PlatformDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [type, setType] = useState<CodePlatform['type']>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [test, setTest] = useState<{ ok: boolean; msg: string } | null>(null);
  const u = url.trim().replace(/\/$/, '');
  const dup = s.platforms.some((p) => p.url === u);
  const runTest = () => {
    if (!/^https:\/\/[\w.-]+\.[a-z]{2,}$/i.test(u)) return setTest({ ok: false, msg: '地址格式应为 https://主机名，不带路径' });
    if (dup) return setTest({ ok: false, msg: '这个地址已经登记过' });
    if (token.trim().length < 8) return setTest({ ok: false, msg: `${type} 拒绝了令牌：令牌无效或已过期` });
    setTest({ ok: true, msg: `已连接 ${type}，令牌为只读权限` });
  };
  const reset = () => setTest(null);
  return (
    <Dialog
      onClose={onClose}
      title="登记代码平台"
      description="保存前先测试连接。"
      footer={
        <>
          <Button className="mr-auto" disabled={!u || !token} onClick={runTest}>测试连接</Button>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" disabled={!test?.ok} title={!test?.ok ? '测试连接通过后才能保存' : undefined} onClick={() => (actions.addPlatform({ id: `p${s.platforms.length + 1}-${u}`, type, url: u, tokenTail: token.trim().slice(-4), addedAt: TODAY }), onClose())}>
            保存
          </Button>
        </>
      }
    >
      <div className="mb-4">
        <div className="mb-2 font-medium">类型</div>
        <Segmented name="ptype" label="代码平台类型" value={type} onChange={(t) => (setType(t), reset())} options={[{ value: 'GitLab', label: 'GitLab' }, { value: 'GitHub', label: 'GitHub' }]} />
      </div>
      <Field label="实例地址" error={test && !test.ok && !test.msg.includes('令牌') ? test.msg : undefined}>
        {(f) => <Input f={f} value={url} onChange={(e) => (setUrl(e.target.value), reset())} placeholder="https://gitlab.corp.example" className="rl-mono" autoFocus />}
      </Field>
      <Field label="只读服务令牌" error={test && !test.ok && test.msg.includes('令牌') ? test.msg : undefined} hint={test?.ok ? <span className="font-medium rl-green">{test.msg}</span> : '保存后只显示末四位'}>
        {(f) => <Input f={f} type="password" value={token} onChange={(e) => (setToken(e.target.value), reset())} autoComplete="off" />}
      </Field>
    </Dialog>
  );
}

export function AdminUsers() {
  const s = useStore();
  const [creating, setCreating] = useState(false);
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<{ user: User; type: IdentityType } | null>(null);
  return (
    <AdminOnly title="用户">
      <PageHead title="用户" sub="用户只能禁用，不能删除。LDAP 用户第一次登录后才会出现在这里。" action={<Button kind="primary" onClick={() => setCreating(true)}>新建本地用户</Button>} />
      <div className="px-8">
        <table className="w-full border-collapse">
          <thead style={{ borderBottom: '1px solid var(--rl-ink)' }}>
            <tr>
              <th className={TH}>用户</th>
              <th className={TH}>邮箱 · 部门</th>
              <th className={TH}>登录身份</th>
              <th className={TH}>状态</th>
              <th className={TH}><span className="sr-only">操作</span></th>
            </tr>
          </thead>
          <tbody>
            {s.users.map((u) => {
              const off = u.status === '已禁用';
              const unbound = (['ldap', 'local'] as const).filter((t) => !u.identities.some((i) => i.type === t));
              const self = u.username === s.me;
              return (
                <tr key={u.username} style={{ borderBottom: '1px solid var(--rl-rule)' }}>
                  <td className="py-3 pr-4">
                    <span className={`rl-t-md font-semibold ${off ? 'rl-ink-3' : ''}`}>{u.displayName}</span>
                    <span className="ml-2 rl-ink-2">{u.username}</span>
                  </td>
                  <td className="py-3 pr-4 rl-ink-2">
                    {u.email || <span className="rl-ink-3">无邮箱</span>}
                    {u.dept && <span className="block">{u.dept}</span>}
                  </td>
                  <td className="py-3 pr-4">
                    {u.identities.map((i) => (
                      <span key={i.type} className="block">
                        {IDENTITY_LABEL[i.type]} <span className="rl-ink-2">{i.login}</span>
                      </span>
                    ))}
                  </td>
                  <td className="py-3 pr-4">{off ? <span className="rl-stamp rl-ink-2">已禁用</span> : '正常'}</td>
                  <td className="py-3 text-right whitespace-nowrap">
                    <span className="inline-flex items-center gap-2">
                      {unbound.map((t) => (
                        <Button key={t} size="sm" onClick={() => setBinding({ user: u, type: t })}>绑定{IDENTITY_LABEL[t]}</Button>
                      ))}
                      {self ? (
                        <span className="w-16 text-center rl-ink-3">当前用户</span>
                      ) : off ? (
                        <Button size="sm" className="w-16" onClick={() => actions.setUserStatus(u.username, '正常')}>启用</Button>
                      ) : (
                        <Button size="sm" className="w-16" onClick={() => setDisabling(u)}>禁用</Button>
                      )}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {creating && <LocalUserDialog onClose={() => setCreating(false)} />}
      {disabling && <DisableDialog user={disabling} onClose={() => setDisabling(null)} />}
      {binding && <BindDialog user={binding.user} type={binding.type} by={`管理员 ${s.me}`} onClose={() => setBinding(null)} />}
    </AdminOnly>
  );
}

function LocalUserDialog({ onClose }: { onClose: () => void }) {
  const [v, setV] = useState<LocalUserInput>({ username: '', displayName: '', email: '', dept: '', password: '' });
  const [tried, setTried] = useState(false);
  const errors = validateLocalUser(v);
  const set = (p: Partial<LocalUserInput>) => setV((x) => ({ ...x, ...p }));
  const err = (k: keyof LocalUserInput) => (tried || (k === 'username' && errors.username === '用户名已存在') ? errors[k] : undefined);
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.createLocalUser(v);
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      title="新建本地用户"
      description="给没有 LDAP 的人（例如外包同事）使用。用户用本地账号密码登录，之后可以在个人设置里绑定 LDAP。"
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>新建</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())} className="grid grid-cols-2 gap-x-4">
        <Field label="用户名" error={err('username')} hint="登录用，小写字母开头">{(f) => <Input f={f} value={v.username} onChange={(e) => set({ username: e.target.value.trim() })} className="rl-mono" autoFocus />}</Field>
        <Field label="显示名" error={err('displayName')}>{(f) => <Input f={f} value={v.displayName} onChange={(e) => set({ displayName: e.target.value })} />}</Field>
        <Field label="邮箱（可选）" error={err('email')}>{(f) => <Input f={f} value={v.email} onChange={(e) => set({ email: e.target.value })} />}</Field>
        <Field label="部门（可选）">{(f) => <Input f={f} value={v.dept} onChange={(e) => set({ dept: e.target.value })} />}</Field>
        <Field className="col-span-2" label="初始密码" error={err('password')} hint="至少 8 位，线下告知本人">{(f) => <Input f={f} type="password" value={v.password} onChange={(e) => set({ password: e.target.value })} autoComplete="new-password" />}</Field>
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

function DisableDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const s = useStore();
  const blocking = soleOwnerOf(s, user.username);
  if (blocking.length)
    return (
      <Dialog
        onClose={onClose}
        title={`不能禁用 ${user.displayName}`}
        description={`下面的应用只有 ${user.displayName} 一个 Owner。先由 Owner 在应用里添加另一位 Owner，再禁用。`}
        footer={<Button onClick={onClose}>知道了</Button>}
      >
        <ul style={{ borderTop: '1px solid var(--rl-ink)' }}>
          {blocking.map((a) => (
            <li key={a.key} className="flex items-baseline justify-between gap-4 py-3" style={{ borderBottom: '1px solid var(--rl-rule)' }}>
              <span>
                <span className="font-semibold">{a.name}</span>
                <Mono className="ml-2 rl-ink-2">{a.key}</Mono>
              </span>
              <LinkButton onClick={() => (onClose(), navigate(`/apps/${a.key}`))}>打开应用</LinkButton>
            </li>
          ))}
        </ul>
      </Dialog>
    );
  return (
    <Dialog
      onClose={onClose}
      title={`禁用 ${user.displayName}？`}
      description="禁用后不能用任何登录身份登录，也不能再被设为 Owner / PE。用户信息和历史记录保留，可以随时重新启用。"
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="danger" onClick={() => (actions.setUserStatus(user.username, '已禁用'), onClose())}>禁用</Button>
        </>
      }
    />
  );
}
