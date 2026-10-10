import { useState } from 'react';
import { actions, soleOwnerOf, TODAY, useStore, type CodePlatform, type User } from '../data';
import { navigate } from '../router';
import { Shell } from '../Shell';
import { Button, Dialog, DialogActions, Field, Input, Mono, PageHeader, Table } from '../ui';
import { BindDialog, IDENTITY_LABEL } from './Settings';

export function AdminPlatforms() {
  const s = useStore();
  const [adding, setAdding] = useState(false);
  return (
    <Shell title="管理后台 / 代码平台">
      <PageHeader title="代码平台" sub="应用建立仓库关联时从这里选择平台。令牌只需要只读权限。" actions={<Button kind="primary" onClick={() => setAdding(true)}>登记代码平台</Button>} />
      <Table head={['类型', '实例地址', '只读服务令牌', '仓库关联', '登记时间']}>
        {s.platforms.map((p) => (
          <tr key={p.id}>
            <td>{p.type}</td>
            <td><Mono>{p.url}</Mono></td>
            <td><Mono className="text-ink-2">••••••••{p.tokenTail}</Mono></td>
            <td>{s.apps.filter((a) => a.repo.platformId === p.id).length} 个应用</td>
            <td className="text-ink-2">{p.addedAt}</td>
          </tr>
        ))}
      </Table>
      {adding && <PlatformDialog onClose={() => setAdding(false)} />}
    </Shell>
  );
}

function PlatformDialog({ onClose }: { onClose: () => void }) {
  const [type, setType] = useState<CodePlatform['type']>('GitLab');
  const [url, setUrl] = useState('');
  const [token, setToken] = useState('');
  const [tested, setTested] = useState<null | 'ok'>(null);
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title="登记代码平台">
      <div className="space-y-4">
        <Field label="类型">
          <div className="flex gap-4">
            {(['GitLab', 'GitHub'] as const).map((t) => (
              <label key={t} className="flex items-center gap-1.5">
                <input type="radio" checked={type === t} onChange={() => setType(t)} />
                {t}
              </label>
            ))}
          </div>
        </Field>
        <Field label="实例地址"><Input value={url} onChange={(e) => (setUrl(e.target.value), setTested(null))} placeholder="https://gitlab.corp.example" /></Field>
        <Field label="只读服务令牌" hint={tested === 'ok' ? '连接成功，令牌只有读权限。' : '保存后只显示末四位。'}>
          <Input type="password" value={token} onChange={(e) => (setToken(e.target.value), setTested(null))} />
        </Field>
      </div>
      <DialogActions>
        <Button kind="quiet" className="mr-auto" disabled={!url || !token} onClick={() => setTested('ok')}>测试连接</Button>
        <Button onClick={onClose}>取消</Button>
        <Button kind="primary" disabled={tested !== 'ok'} onClick={() => (actions.addPlatform({ id: `p${Date.now()}`, type, url, tokenTail: token.slice(-4), addedAt: TODAY }), onClose())}>
          保存
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export function AdminUsers() {
  const s = useStore();
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<{ user: User; type: 'ldap' | 'local' } | null>(null);
  return (
    <Shell title="管理后台 / 用户">
      <PageHeader title="用户" sub="用户只能禁用，不能删除。LDAP 用户需要先登录一次才会出现在这里。" actions={<Button kind="primary">新建本地用户</Button>} />
      <Table head={['用户', '邮箱', '部门', '登录身份', '状态', '']}>
        {s.users.map((u) => {
          const unbound = (['ldap', 'local'] as const).filter((t) => !u.identities.some((i) => i.type === t));
          const off = u.status === '已禁用';
          return (
            <tr key={u.username} className={off ? 'text-ink-3' : ''}>
              <td>
                <div className="font-medium">{u.displayName}</div>
                <Mono className="text-ink-2">{u.username}</Mono>
              </td>
              <td>{u.email || '—'}</td>
              <td>{u.dept ?? '—'}</td>
              <td>{u.identities.map((i) => IDENTITY_LABEL[i.type]).join('、')}</td>
              <td>{off ? '已禁用' : '正常'}</td>
              <td className="text-right whitespace-nowrap">
                {unbound.map((t) => (
                  <Button key={t} kind="quiet" className="mr-4" onClick={() => setBinding({ user: u, type: t })}>绑定{IDENTITY_LABEL[t]}</Button>
                ))}
                {u.username !== s.me && (off ? <Button onClick={() => actions.setUserStatus(u.username, '正常')}>启用</Button> : <Button onClick={() => setDisabling(u)}>禁用</Button>)}
              </td>
            </tr>
          );
        })}
      </Table>
      {disabling && <DisableDialog user={disabling} onClose={() => setDisabling(null)} />}
      {binding && <BindDialog user={binding.user} type={binding.type} by="管理员绑定" onClose={() => setBinding(null)} />}
    </Shell>
  );
}

function DisableDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const s = useStore();
  const blocking = soleOwnerOf(s, user.username);
  if (blocking.length)
    return (
      <Dialog open onOpenChange={(o) => !o && onClose()} title={`不能禁用 ${user.displayName}`} description={`下面这些应用只有 ${user.displayName} 一个 Owner。需要先由 Owner 在应用里添加其他 Owner，再禁用。`}>
        <ul className="divide-y divide-line-2 border-y border-line">
          {blocking.map((a) => (
            <li key={a.key} className="flex items-baseline justify-between py-2.5">
              <span>{a.name}<Mono className="ml-2 text-ink-2">{a.key}</Mono></span>
              <Button kind="quiet" onClick={() => navigate(`/apps/${a.key}`)}>查看应用</Button>
            </li>
          ))}
        </ul>
        <DialogActions><Button onClick={onClose}>知道了</Button></DialogActions>
      </Dialog>
    );
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={`禁用 ${user.displayName}？`} description="禁用后不能用任何登录身份登录。用户信息和历史记录保留，可以随时重新启用。">
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind="danger" onClick={() => (actions.setUserStatus(user.username, '已禁用'), onClose())}>禁用</Button>
      </DialogActions>
    </Dialog>
  );
}
