import { useState } from 'react';
import { actions, can, useStore, userName } from '../data';
import { VARIANTS } from '../lineage';
import { ForkDialog, VersionDialog } from '../lineage/Dialogs';
import { navigate, setParam, useLocation } from '../router';
import { Shell } from '../Shell';
import { Button, Mono, PageHeader, SectionTitle, Textarea } from '../ui';

export function AppDetail({ appKey }: { appKey: string }) {
  const s = useStore();
  const { params } = useLocation();
  const [forkFrom, setForkFrom] = useState<string | null>(null);
  const [registerOn, setRegisterOn] = useState<string | null>(null);
  const app = s.apps.find((a) => a.key === appKey);
  if (!app) return <Shell title="应用">没有这个应用。</Shell>;

  const perm = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId)!;
  const variant = VARIANTS[params.get('variant') ?? ''] ? params.get('variant')! : 'A';
  const Lineage = VARIANTS[variant].Component;
  const selected = params.get('line') ?? `${appKey}:main`;

  return (
    <Shell
      title={
        <>
          <a href="/apps" onClick={(e) => (e.preventDefault(), navigate('/apps'))} className="hover:text-ink">应用</a>
          <span className="mx-2 text-ink-3">/</span>
          <span className="text-ink">{app.name}</span>
        </>
      }
    >
      <PageHeader
        title={app.name}
        sub={app.description}
        actions={
          <>
            {perm.editApp && <Button>编辑应用</Button>}
            {perm.admin && <Button kind="danger">删除应用</Button>}
          </>
        }
      />

      <dl className="mb-12 grid grid-cols-[auto_1fr_auto_1fr] gap-x-6 gap-y-2.5 text-[13px] [&>dt]:text-ink-2">
        <dt>应用标识</dt>
        <dd><Mono>{app.key}</Mono><span className="ml-2 text-ink-3">创建后不可修改</span></dd>
        <dt>Owner</dt>
        <dd>{app.owners.map(userName).join('、')}</dd>
        <dt>仓库关联</dt>
        <dd>
          <Mono>{app.repo.path}</Mono>
          <span className="ml-2 text-ink-3">{platform.type} {new URL(platform.url).host} · 项目 ID {app.repo.projectId}</span>
        </dd>
        <dt>PE</dt>
        <dd>{app.pes.length ? app.pes.map(userName).join('、') : <span className="text-ink-3">未设置</span>}</dd>
        <dt>主线分支名</dt>
        <dd><Mono>{app.mainBranch}</Mono></dd>
      </dl>

      <section className="mb-14">
        <SectionTitle actions={perm.manageLines && <Button onClick={() => setForkFrom('')}>分叉维护线</Button>}>分支谱系</SectionTitle>
        <Lineage
          appKey={app.key}
          selected={selected}
          onSelect={(id) => setParam('line', id)}
          onFork={(vid) => setForkFrom(vid)}
          onRegister={(lid) => setRegisterOn(lid)}
        />
      </section>

      <div className="grid grid-cols-2 gap-12">
        <DeployInfo appKey={app.key} value={app.deployInfo} editable={perm.editDeploy} />
        <section>
          <SectionTitle>变更历史</SectionTitle>
          <ul className="space-y-2 text-[13px]">
            {s.audit.filter((e) => e.appKey === app.key).map((e, i) => (
              <li key={i} className="flex gap-4">
                <span className="shrink-0 whitespace-nowrap text-ink-3">{e.at}</span>
                <span className="w-12 shrink-0 text-ink-2">{userName(e.actor)}</span>
                <span>{e.text}</span>
              </li>
            ))}
          </ul>
        </section>
      </div>

      {forkFrom !== null && (
        <ForkDialog key={forkFrom} appKey={app.key} sourceVersionId={forkFrom} onClose={() => setForkFrom(null)} onCreated={(id) => setParam('line', id)} />
      )}
      {registerOn && <VersionDialog key={registerOn} lineId={registerOn} onClose={() => setRegisterOn(null)} />}
    </Shell>
  );
}

function DeployInfo({ appKey, value, editable }: { appKey: string; value: string; editable: boolean }) {
  const [draft, setDraft] = useState<string | null>(null);
  return (
    <section>
      <SectionTitle actions={editable && draft === null && <Button kind="quiet" onClick={() => setDraft(value)}>编辑</Button>}>部署相关信息</SectionTitle>
      {draft === null ? (
        value ? <p className="whitespace-pre-line text-[13px] leading-relaxed">{value}</p> : <p className="text-[13px] text-ink-3">还没有填写。PE 和 Owner 可以编辑。</p>
      ) : (
        <>
          <Textarea rows={5} value={draft} onChange={(e) => setDraft(e.target.value)} />
          <div className="mt-2 flex gap-2">
            <Button kind="primary" onClick={() => (actions.setDeployInfo(appKey, draft), setDraft(null))}>保存</Button>
            <Button onClick={() => setDraft(null)}>取消</Button>
          </div>
        </>
      )}
    </section>
  );
}
