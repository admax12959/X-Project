import { useState } from 'react';
import { actions, can, soleOwnerOf, TODAY, useStore, validateLocalUser, type CodePlatform, type LocalUserInput, type User } from '../../data';
import { navigate } from '../../router';
import { BindDialog, IDENTITY_LABEL } from './Settings';
import { Crumbs, NoAccess, PageHead, Shell } from './Shell';
import { Button, Code, Dialog, DialogActions, Field, Input, Seg } from './ui';

// ---------- 代码平台 ----------

export function AdminPlatforms() {
  const s = useStore();
  const [adding, setAdding] = useState(false);
  const crumbs = <Crumbs items={[{ label: '管理后台' }, { label: '代码平台' }]} />;
  if (!can(s.role).admin) return <Shell crumbs={crumbs}><NoAccess what="代码平台管理" /></Shell>;
  return (
    <Shell crumbs={crumbs}>
      <PageHead
        title="代码平台"
        sub="应用建立仓库关联时从这里选择平台。工具只用只读服务令牌访问仓库，保存后只显示末四位。"
        actions={<Button kind="primary" onClick={() => setAdding(true)}>登记代码平台</Button>}
      />
      <ul className="border-t-2 border-[var(--lc-ink)]">
        {s.platforms.map((p) => {
          const apps = s.apps.filter((a) => a.repo.platformId === p.id);
          return (
            <li key={p.id} className="grid grid-cols-12 items-baseline gap-x-6 gap-y-2 border-b border-[var(--lc-rule)] py-6">
              <div className="col-span-4 min-w-0 max-[1100px]:col-span-12">
                <div className="flex items-baseline gap-3">
                  <span className="truncate text-[length:var(--lc-text-lg)] leading-tight font-bold">
                    <Code>{new URL(p.url).host}</Code>
                  </span>
                  <span className="shrink-0 text-[length:var(--lc-text-sm)] font-semibold text-[var(--lc-ink-2)]">{p.type}</span>
                </div>
                <div className="lc-code mt-1 truncate text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">{p.url}</div>
              </div>
              <dl className="col-span-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 text-[length:var(--lc-text-sm)] max-[1100px]:col-span-5">
                <dt className="lc-muted">只读令牌</dt>
                <dd><Code>••••••••{p.tokenTail}</Code></dd>
                <dt className="lc-muted">登记于</dt>
                <dd className="lc-num">{p.addedAt}</dd>
              </dl>
              <div className="col-span-5 max-[1100px]:col-span-7">
                <div className="lc-muted text-[length:var(--lc-text-sm)]">仓库关联 · {apps.length} 个应用</div>
                {apps.length ? (
                  <ul className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
                    {apps.map((a) => (
                      <li key={a.key}>
                        <a href={`/apps/${a.key}`} onClick={(e) => (e.preventDefault(), navigate(`/apps/${a.key}`))} className="lc-link">
                          {a.name}
                        </a>
                        <Code className="ml-1.5 text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">{a.repo.path}</Code>
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="mt-1 text-[var(--lc-ink-2)]">还没有应用使用这个平台。</p>
                )}
              </div>
            </li>
          );
        })}
      </ul>
      {adding && <PlatformDialog onClose={() => setAdding(false)} />}
    </Shell>
  );
}

function PlatformDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [type, setType] = useState<CodePlatform['type']>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [test, setTest] = useState<null | { ok: boolean; urlErr?: string; tokenErr?: string }>(null);
  const reset = () => setTest(null);
  const host = (() => {
    try {
      return new URL(url.trim()).host;
    } catch {
      return '';
    }
  })();

  const runTest = () => {
    const u = url.trim().replace(/\/+$/, '');
    const urlErr = !/^https?:\/\/[^\s/]+$/.test(u) ? '地址格式应为 https://主机名，不带路径' : s.platforms.some((p) => p.url === u) ? '这个实例已经登记过' : undefined;
    const tokenErr = token.length < 8 ? `${type} 拒绝了这个令牌：长度不足，检查是否复制完整` : undefined;
    setTest({ ok: !urlErr && !tokenErr, urlErr, tokenErr });
  };

  return (
    <Dialog onClose={onClose} title="登记代码平台" description="先测试连接，通过后才能保存。令牌保存后只显示末四位。">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (!test?.ok) return;
          actions.addPlatform({ id: `p${s.platforms.length + 1}-${host}`, type, url: url.trim().replace(/\/+$/, ''), tokenTail: token.slice(-4), addedAt: TODAY });
          onClose();
        }}
        noValidate
      >
        <div className="lc-field-label">类型</div>
        <Seg label="类型" value={type} options={[{ value: 'GitLab', label: 'GitLab' }, { value: 'GitHub', label: 'GitHub' }]} onChange={(t) => (setType(t), reset())} />
        <Field className="mt-4" label="实例地址" error={test?.urlErr} help="例如 https://gitlab.corp.example">
          <Input value={url} onChange={(e) => (setUrl(e.target.value), reset())} className="lc-code" autoFocus />
        </Field>
        <Field
          className="mt-2"
          label="只读服务令牌"
          error={test?.tokenErr}
          help={test?.ok ? <span className="font-semibold text-[var(--lc-ink)]">连接成功：{host} 可访问，令牌只有读权限。</span> : '只需要读取仓库信息的权限'}
        >
          <Input type="password" value={token} onChange={(e) => (setToken(e.target.value), reset())} autoComplete="off" />
        </Field>
        <DialogActions
          left={
            <Button onClick={runTest} disabled={!url.trim() || !token}>
              测试连接
            </Button>
          }
        >
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit" disabled={!test?.ok} title={test?.ok ? undefined : '先测试连接'}>
            保存
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

// ---------- 用户 ----------

export function AdminUsers() {
  const s = useStore();
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<{ user: User; type: 'ldap' | 'local' } | null>(null);
  const [creating, setCreating] = useState(false);
  const crumbs = <Crumbs items={[{ label: '管理后台' }, { label: '用户' }]} />;
  if (!can(s.role).admin) return <Shell crumbs={crumbs}><NoAccess what="用户管理" /></Shell>;

  return (
    <Shell crumbs={crumbs}>
      <PageHead
        title="用户"
        sub="用户只能禁用，不能删除。LDAP 用户第一次登录后才会出现在这里。禁用前先看右侧：唯一 Owner 的用户不能直接禁用。"
        actions={<Button kind="primary" onClick={() => setCreating(true)}>新建本地用户</Button>}
      />
      <div className="grid grid-cols-12 gap-x-6 border-b-2 border-[var(--lc-ink)] pb-2">
        <span className="lc-label col-span-3">用户</span>
        <span className="lc-label col-span-3">登录身份</span>
        <span className="lc-label col-span-4">负责的应用</span>
        <span className="lc-label col-span-2 text-right">状态</span>
      </div>
      <ul>
        {s.users.map((u) => {
          const off = u.status === '已禁用';
          const unbound = (['ldap', 'local'] as const).filter((t) => !u.identities.some((i) => i.type === t));
          const owns = s.apps.filter((a) => a.owners.includes(u.username));
          const pe = s.apps.filter((a) => a.pes.includes(u.username));
          const sole = new Set(soleOwnerOf(s, u.username).map((a) => a.key));
          return (
            <li key={u.username} className={`grid grid-cols-12 items-start gap-x-6 gap-y-2 border-b border-[var(--lc-rule)] py-5 ${off ? 'bg-[var(--lc-paper-2)]' : ''}`}>
              <div className="col-span-3 min-w-0">
                <div className={`text-[length:var(--lc-text-md)] leading-tight font-bold ${off ? 'text-[var(--lc-muted)]' : ''}`}>{u.displayName}</div>
                <div className="mt-1 truncate text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                  <Code>{u.username}</Code>
                  {u.dept && <> · {u.dept}</>}
                </div>
                {u.email && <div className="lc-code truncate text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">{u.email}</div>}
              </div>
              <div className="col-span-3 text-[length:var(--lc-text-sm)]">
                <ul className="space-y-0.5">
                  {u.identities.map((i) => (
                    <li key={i.type}>
                      <span className="font-semibold">{IDENTITY_LABEL[i.type]}</span> <Code className="text-[var(--lc-ink-2)]">{i.login}</Code>
                    </li>
                  ))}
                </ul>
                {unbound.map((t) => (
                  <Button key={t} kind="ghost" className="!h-7 !px-0" onClick={() => setBinding({ user: u, type: t })}>
                    绑定{IDENTITY_LABEL[t]}
                  </Button>
                ))}
              </div>
              <div className="col-span-4 text-[length:var(--lc-text-sm)]">
                {owns.length + pe.length === 0 ? (
                  <span className="text-[var(--lc-muted)]">不负责任何应用</span>
                ) : (
                  <dl className="grid grid-cols-[52px_minmax(0,1fr)] gap-x-2 gap-y-0.5">
                    {owns.length > 0 && (
                      <>
                        <dt className="lc-muted">Owner</dt>
                        <dd>
                          {owns.map((a, i) => (
                            <span key={a.key}>
                              {i > 0 && '、'}
                              {a.name}
                              {sole.has(a.key) && <span className="font-bold">（唯一 Owner）</span>}
                            </span>
                          ))}
                        </dd>
                      </>
                    )}
                    {pe.length > 0 && (
                      <>
                        <dt className="lc-muted">PE</dt>
                        <dd>{pe.map((a) => a.name).join('、')}</dd>
                      </>
                    )}
                  </dl>
                )}
              </div>
              <div className="col-span-2 flex flex-col items-end gap-2">
                <span className={off ? 'font-bold text-[var(--lc-ink-2)]' : 'text-[var(--lc-ink-2)]'}>{off ? '已禁用' : '正常'}</span>
                {u.username === s.me ? (
                  <span className="text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">当前用户</span>
                ) : off ? (
                  <Button className="!h-8" onClick={() => actions.setUserStatus(u.username, '正常')} aria-label={`启用 ${u.displayName}`}>
                    启用
                  </Button>
                ) : (
                  <Button className="!h-8" onClick={() => setDisabling(u)} aria-label={`禁用 ${u.displayName}`}>
                    禁用
                  </Button>
                )}
              </div>
            </li>
          );
        })}
      </ul>
      {disabling && <DisableDialog user={disabling} onClose={() => setDisabling(null)} />}
      {binding && <BindDialog user={binding.user} type={binding.type} by={`管理员 ${s.me}`} onClose={() => setBinding(null)} />}
      {creating && <CreateUserDialog onClose={() => setCreating(false)} />}
    </Shell>
  );
}

function DisableDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const s = useStore();
  const blocking = soleOwnerOf(s, user.username);
  if (blocking.length)
    return (
      <Dialog onClose={onClose} title={`不能禁用 ${user.displayName}`} description={`下面的应用只有 ${user.displayName} 一个 Owner。先由 Owner 在应用里添加其他 Owner，再来禁用。`}>
        <ul className="border-y border-[var(--lc-ink)]">
          {blocking.map((a) => (
            <li key={a.key} className="flex items-baseline justify-between gap-4 border-b border-[var(--lc-rule)] py-3 last:border-b-0">
              <span>
                <span className="font-bold">{a.name}</span>
                <Code className="ml-2 text-[var(--lc-muted)]">{a.key}</Code>
              </span>
              <a href={`/apps/${a.key}`} onClick={(e) => (e.preventDefault(), navigate(`/apps/${a.key}`))} className="lc-link text-[length:var(--lc-text-sm)]">
                打开应用
              </a>
            </li>
          ))}
        </ul>
        <DialogActions>
          <Button onClick={onClose} autoFocus>
            知道了
          </Button>
        </DialogActions>
      </Dialog>
    );
  return (
    <Dialog onClose={onClose} title={`禁用 ${user.displayName}？`} description="禁用后不能用任何登录身份登录，也不能再被设为 Owner 或 PE。用户信息和历史记录保留，可以随时重新启用。">
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind="danger" autoFocus onClick={() => (actions.setUserStatus(user.username, '已禁用'), onClose())}>
          禁用
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function CreateUserDialog({ onClose }: { onClose: () => void }) {
  const [f, setF] = useState<LocalUserInput>({ username: '', displayName: '', email: '', dept: '', password: '' });
  const [tried, setTried] = useState(false);
  const set = (patch: Partial<LocalUserInput>) => setF((x) => ({ ...x, ...patch }));
  const errors = validateLocalUser(f);
  const err = (k: keyof LocalUserInput) => (tried ? errors[k] : undefined);
  return (
    <Dialog onClose={onClose} title="新建本地用户" description="给没有 LDAP 的人（例如外包同事）用。登录身份是本地账号密码，初始密码由你线下告知本人。">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setTried(true);
          if (Object.keys(errors).length) return;
          actions.createLocalUser(f);
          onClose();
        }}
        noValidate
      >
        <div className="grid grid-cols-2 gap-x-4">
          <Field label="用户名" error={err('username')} help="小写字母开头，2–32 位">
            <Input value={f.username} onChange={(e) => set({ username: e.target.value.trim() })} className="lc-code" autoFocus />
          </Field>
          <Field label="显示名" error={err('displayName')}>
            <Input value={f.displayName} onChange={(e) => set({ displayName: e.target.value })} />
          </Field>
          <Field label="邮箱" error={err('email')} help="可选">
            <Input value={f.email} onChange={(e) => set({ email: e.target.value })} className="lc-code" />
          </Field>
          <Field label="部门" help="可选">
            <Input value={f.dept} onChange={(e) => set({ dept: e.target.value })} />
          </Field>
        </div>
        <Field label="初始密码" error={err('password')} help="至少 8 位">
          <Input type="password" value={f.password} onChange={(e) => set({ password: e.target.value })} autoComplete="new-password" />
        </Field>
        <DialogActions left={tried && Object.keys(errors).length > 0 && <span className="text-[length:var(--lc-text-sm)] font-semibold text-[var(--lc-error)]">还有 {Object.keys(errors).length} 处需要修改</span>}>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" type="submit">
            创建
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
