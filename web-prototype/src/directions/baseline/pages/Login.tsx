import { Tabs } from '@base-ui/react/tabs';
import { navigate } from '../../../router';
import { Button, Field, Input } from '../ui';

export function Login() {
  const go = (e: React.FormEvent) => (e.preventDefault(), navigate('/apps'));
  return (
    <div className="flex min-h-screen items-center justify-center bg-wash">
      <div className="w-[360px]">
        <h1 className="mb-1 text-[22px] font-semibold">应用维护工具</h1>
        <p className="mb-6 text-ink-2">登记应用、Owner/PE、维护线和版本</p>
        <Tabs.Root defaultValue="ldap" className="border border-line bg-paper">
          <Tabs.List className="flex border-b border-line">
            {[
              ['ldap', 'LDAP'],
              ['local', '本地账号密码'],
            ].map(([v, label]) => (
              <Tabs.Tab key={v} value={v} className="flex-1 border-b-2 border-transparent py-2.5 text-ink-2 data-[active]:border-accent data-[active]:font-medium data-[active]:text-ink">
                {label}
              </Tabs.Tab>
            ))}
          </Tabs.List>
          <Tabs.Panel value="ldap">
            <form onSubmit={go} className="space-y-4 p-6">
              <Field label="LDAP 用户名" hint="首次登录会自动创建用户，显示名、邮箱、部门从 LDAP 同步">
                <Input autoFocus defaultValue="linqiao" />
              </Field>
              <Field label="密码">
                <Input type="password" defaultValue="........" />
              </Field>
              <Button kind="primary" className="w-full">登录</Button>
            </form>
          </Tabs.Panel>
          <Tabs.Panel value="local">
            <form onSubmit={go} className="space-y-4 p-6">
              <Field label="用户名" hint="本地账号密码由管理员创建，或在个人设置里自行绑定">
                <Input />
              </Field>
              <Field label="密码">
                <Input type="password" />
              </Field>
              <Button kind="primary" className="w-full">登录</Button>
            </form>
          </Tabs.Panel>
        </Tabs.Root>
      </div>
    </div>
  );
}
