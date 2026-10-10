import { Tabs } from '@base-ui/react/tabs';
import { useState, type FormEvent } from 'react';
import { TODAY } from '../../data';
import { navigate } from '../../router';
import { Button, Field, Input } from './ui';

function LoginForm({ kind }: { kind: 'ldap' | 'local' }) {
  const [user, setUser] = useState('');
  const [pw, setPw] = useState('');
  const [tried, setTried] = useState(false);
  const errors = { user: user.trim() ? undefined : kind === 'ldap' ? '填写 LDAP 用户名' : '填写用户名', pw: pw ? undefined : '填写密码' };
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setTried(true);
    if (errors.user || errors.pw) return;
    navigate('/apps');
  };
  return (
    <form onSubmit={submit} noValidate className="min-h-[264px] pt-6">
      <Field label={kind === 'ldap' ? 'LDAP 用户名' : '用户名'} error={tried ? errors.user : undefined} help={kind === 'ldap' ? '首次登录会自动创建用户，显示名、邮箱、部门从 LDAP 同步' : '本地账号密码由管理员创建，或在个人设置里自行绑定'}>
        <Input value={user} onChange={(e) => setUser(e.target.value)} autoComplete="username" />
      </Field>
      <Field className="mt-2" label="密码" error={tried ? errors.pw : undefined}>
        <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="current-password" />
      </Field>
      <Button kind="primary" type="submit" className="mt-4 w-full">
        登录
      </Button>
    </form>
  );
}

export function Login() {
  return (
    <div className="relative min-h-screen bg-[var(--lc-paper)] text-[var(--lc-ink)]">
      {/* the exposed 12-column grid, shown once, on the one page with nothing else to carry it */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 mx-auto max-w-[1280px]"
        style={{ backgroundImage: 'repeating-linear-gradient(to right, var(--lc-rule) 0, var(--lc-rule) 1px, transparent 1px, transparent calc(100% / 12))' }}
      />
      <div className="relative mx-auto grid min-h-screen max-w-[1280px] grid-cols-12 items-center gap-x-0 px-0">
        <div className="col-span-6 col-start-2 max-[900px]:col-span-10 max-[900px]:col-start-2">
          <h1 className="text-[length:var(--lc-text-poster)] leading-[1.02] font-extrabold tracking-tight max-[900px]:text-[length:var(--lc-text-display)]">
            应用维护工具<span aria-hidden className="ml-1 inline-block h-[0.32em] w-[0.32em] bg-[var(--lc-ink)]" />
          </h1>
          <p className="mt-6 max-w-[34ch] text-[length:var(--lc-text-md)] leading-relaxed text-[var(--lc-ink-2)]">登记应用、Owner 和 PE，记录每条维护线从分叉到终止的时间和版本。</p>
          <p className="lc-num mt-10 border-t-2 border-[var(--lc-ink)] pt-3 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
            今天 <span className="font-bold text-[var(--lc-signal)]">{TODAY}</span>
          </p>
        </div>
        <div className="col-span-3 col-start-9 bg-[var(--lc-paper)] max-[900px]:col-span-10 max-[900px]:col-start-2 max-[900px]:pb-16">
          <Tabs.Root defaultValue="ldap">
            <Tabs.List className="lc-seg !flex w-full" aria-label="登录身份">
              {[
                ['ldap', 'LDAP'],
                ['local', '本地账号密码'],
              ].map(([v, label]) => (
                <Tabs.Tab key={v} value={v} className="flex-1 !text-[var(--lc-ink-2)] data-[active]:!bg-[var(--lc-ink)] data-[active]:!text-[var(--lc-paper)]">
                  {label}
                </Tabs.Tab>
              ))}
            </Tabs.List>
            <Tabs.Panel value="ldap">
              <LoginForm kind="ldap" />
            </Tabs.Panel>
            <Tabs.Panel value="local">
              <LoginForm kind="local" />
            </Tabs.Panel>
          </Tabs.Root>
        </div>
      </div>
    </div>
  );
}
