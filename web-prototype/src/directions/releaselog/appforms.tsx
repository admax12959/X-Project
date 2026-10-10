import { useState } from 'react';
import { actions, useStore, validateApp, verifyRepo, type AppInput, type Application } from '../../data';
import { navigate } from '../../router';
import { Button, Dialog, Field, Input, Mono, PeoplePicker, Select, Textarea } from './ui';

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const s = useStore();
  const [v, setV] = useState<AppInput>({ key: '', name: '', description: '', platformId: s.platforms[0]?.id ?? '', repoPath: '', projectId: '', mainBranch: 'main', owners: [], pes: [] });
  const [repoMsg, setRepoMsg] = useState<string | null>(null);
  const [tried, setTried] = useState(false);
  const set = (p: Partial<AppInput>) => setV((x) => ({ ...x, ...p }));
  const errors = validateApp(v, 'create');
  const keyTaken = errors.key === '应用标识已被占用';
  const err = (k: keyof AppInput) => (tried || (k === 'key' && keyTaken) ? errors[k] : undefined);

  const check = () => {
    const r = verifyRepo(v.platformId, v.repoPath);
    if (r.ok) (set({ projectId: r.projectId }), setRepoMsg(null));
    else (set({ projectId: '' }), setRepoMsg(r.message));
  };
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.createApp(v);
    onClose();
    navigate(`/apps/${v.key}`);
  };
  const platform = s.platforms.find((p) => p.id === v.platformId);

  return (
    <Dialog
      onClose={onClose}
      width={640}
      title="创建应用"
      description="创建后会自动建立主线。应用标识和仓库关联创建后不能修改。"
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>创建应用</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), submit())} className="grid grid-cols-2 gap-x-4">
        <Field label="应用标识" error={err('key')} hint="小写字母开头，可含数字和 -，创建后不能改">
          {(f) => <Input f={f} value={v.key} onChange={(e) => set({ key: e.target.value.trim() })} placeholder="billing-core" className="rl-mono" autoFocus />}
        </Field>
        <Field label="显示名" error={err('name')}>{(f) => <Input f={f} value={v.name} onChange={(e) => set({ name: e.target.value })} placeholder="计费核心" />}</Field>
        <Field className="col-span-2" label="描述（可选）">{(f) => <Textarea f={f} rows={2} value={v.description} onChange={(e) => set({ description: e.target.value })} />}</Field>

        <fieldset className="col-span-2 mb-1 pt-4" style={{ borderTop: '1px solid var(--rl-rule)' }}>
          <legend className="sr-only">仓库关联</legend>
          <div className="mb-3 font-semibold">仓库关联</div>
          <div className="grid grid-cols-[220px_minmax(0,1fr)_auto] items-start gap-x-3">
            <Field label="代码平台" error={err('platformId')}>
              {(f) => (
                <Select f={f} value={v.platformId} onChange={(e) => (set({ platformId: e.target.value, projectId: '' }), setRepoMsg(null))}>
                  <option value="">选择</option>
                  {s.platforms.map((p) => (
                    <option key={p.id} value={p.id}>{p.type} · {new URL(p.url).host}</option>
                  ))}
                </Select>
              )}
            </Field>
            <Field
              label="仓库路径"
              error={repoMsg ?? err('repoPath')}
              hint={v.projectId ? <span className="font-medium rl-green">已找到仓库，项目 ID {v.projectId}</span> : platform ? `${platform.url}/…` : undefined}
            >
              {(f) => <Input f={f} value={v.repoPath} onChange={(e) => (set({ repoPath: e.target.value, projectId: '' }), setRepoMsg(null))} placeholder="trade/billing-core" className="rl-mono" />}
            </Field>
            <div className="pt-7">
              <Button onClick={check} disabled={!v.platformId || !v.repoPath.trim()}>校验</Button>
            </div>
          </div>
        </fieldset>

        <Field label="主线分支名" error={err('mainBranch')} hint="默认 main，可以是 develop 等">
          {(f) => <Input f={f} value={v.mainBranch} onChange={(e) => set({ mainBranch: e.target.value })} className="rl-mono" />}
        </Field>
        <div />
        <Field className="col-span-2" label="Owner" error={err('owners')} hint="至少一个。只能选状态正常的用户">
          {(f) => <PeoplePicker f={f} value={v.owners} onChange={(owners) => set({ owners })} users={s.users} exclude={v.pes} addLabel="添加 Owner…" />}
        </Field>
        <Field className="col-span-2" label="PE（可选）">
          {(f) => <PeoplePicker f={f} value={v.pes} onChange={(pes) => set({ pes })} users={s.users} exclude={v.owners} addLabel="添加 PE…" />}
        </Field>
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

export function EditAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const s = useStore();
  const [v, setV] = useState({ name: app.name, description: app.description, mainBranch: app.mainBranch, owners: app.owners, pes: app.pes });
  const [tried, setTried] = useState(false);
  const set = (p: Partial<typeof v>) => setV((x) => ({ ...x, ...p }));
  const input: AppInput = { ...v, key: app.key, platformId: app.repo.platformId, repoPath: app.repo.path, projectId: app.repo.projectId };
  const errors = validateApp(input, 'edit');
  const err = (k: keyof AppInput) => (tried ? errors[k] : undefined);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const submit = () => {
    setTried(true);
    if (Object.keys(errors).length) return;
    actions.updateApp(app.key, v);
    onClose();
  };
  return (
    <Dialog
      onClose={onClose}
      width={620}
      title={`编辑「${app.name}」`}
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="primary" onClick={submit}>保存</Button>
        </>
      }
    >
      <dl className="mb-5 grid grid-cols-[72px_minmax(0,1fr)] gap-x-3 gap-y-2 rl-bg-2 px-4 py-3" style={{ border: '1px solid var(--rl-rule)' }}>
        <dt className="rl-ink-3">应用标识</dt>
        <dd><Mono className="rl-ink">{app.key}</Mono></dd>
        <dt className="rl-ink-3">仓库关联</dt>
        <dd><Mono className="rl-ink">{app.repo.path}</Mono><span className="ml-2 rl-ink-3">{platform?.type} · 项目 ID {app.repo.projectId}</span></dd>
        <dd className="col-span-2 rl-t-xs rl-ink-3">这两项创建后不能修改。</dd>
      </dl>
      <form onSubmit={(e) => (e.preventDefault(), submit())} className="grid grid-cols-2 gap-x-4">
        <Field label="显示名" error={err('name')}>{(f) => <Input f={f} value={v.name} onChange={(e) => set({ name: e.target.value })} autoFocus />}</Field>
        <Field label="主线分支名" error={err('mainBranch')}>{(f) => <Input f={f} value={v.mainBranch} onChange={(e) => set({ mainBranch: e.target.value })} className="rl-mono" />}</Field>
        <Field className="col-span-2" label="描述">{(f) => <Textarea f={f} rows={2} value={v.description} onChange={(e) => set({ description: e.target.value })} />}</Field>
        <Field className="col-span-2" label="Owner" error={err('owners')} hint="至少保留一个 Owner。增删会写入变更历史">
          {(f) => <PeoplePicker f={f} value={v.owners} onChange={(owners) => set({ owners })} users={s.users} exclude={v.pes} addLabel="添加 Owner…" minOne />}
        </Field>
        <Field className="col-span-2" label="PE">
          {(f) => <PeoplePicker f={f} value={v.pes} onChange={(pes) => set({ pes })} users={s.users} exclude={v.owners} addLabel="添加 PE…" />}
        </Field>
        <button type="submit" hidden />
      </form>
    </Dialog>
  );
}

export function DeleteAppDialog({ app, onClose }: { app: Application; onClose: () => void }) {
  const [typed, setTyped] = useState('');
  const ok = typed.trim() === app.key;
  const go = () => {
    if (!ok) return;
    onClose();
    actions.deleteApp(app.key);
    navigate('/apps');
  };
  return (
    <Dialog
      onClose={onClose}
      title={`删除应用「${app.name}」？`}
      description="应用、它的主线、全部维护线和已登记的版本都会被删除，不能恢复。代码仓库本身不受影响。"
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button kind="danger" disabled={!ok} onClick={go}>删除应用</Button>
        </>
      }
    >
      <form onSubmit={(e) => (e.preventDefault(), go())}>
        <Field label={<>输入应用标识 <Mono className="rl-ink">{app.key}</Mono> 确认</>} hint="输入一致后才能删除">
          {(f) => <Input f={f} value={typed} onChange={(e) => setTyped(e.target.value)} className="rl-mono" autoFocus autoComplete="off" />}
        </Field>
      </form>
    </Dialog>
  );
}
