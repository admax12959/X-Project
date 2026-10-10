import { useState } from 'react';
import { actions, appVersions, lineageOrder, lineById, lineVersions, TODAY, useStore } from '../data';
import { Button, Dialog, DialogActions, Field, Input, Mono, Textarea } from '../ui';

// Open question (flagged to the user): can you fork from a version on a 终止 line?
// Scope says "任何线上的任何版本都可以作为源版本", so the prototype allows it.

export function ForkDialog({ appKey, sourceVersionId, onClose, onCreated }: { appKey: string; sourceVersionId: string | null; onClose: () => void; onCreated: (lineId: string) => void }) {
  const s = useStore();
  const [src, setSrc] = useState(sourceVersionId ?? '');
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);

  const branchTaken = s.lines.some((l) => l.appKey === appKey && l.branch === branch.trim());
  const errors = {
    src: !src ? '选择源版本' : undefined,
    name: !name.trim() ? '填写名称' : undefined,
    branch: !branch.trim() ? '填写 Git 分支名' : branchTaken ? '本应用已有维护线使用这个分支名' : undefined,
    reason: !reason.trim() ? '分叉原因必填，写清楚为什么不能在主线上解决' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);

  const submit = () => {
    setTried(true);
    if (!ok) return;
    const id = actions.fork({ appKey, name: name.trim(), branch: branch.trim(), sourceVersionId: src, reason: reason.trim(), plannedEnd: plannedEnd || undefined, note: note.trim() || undefined });
    onCreated(id);
    onClose();
  };

  return (
    <Dialog open={sourceVersionId !== null} onOpenChange={(o) => !o && onClose()} title="分叉维护线" description="新维护线从源版本开始独立演进，状态为活跃。Owner / PE 沿用应用的设置。" width={540}>
      <div className="space-y-4">
        <Field label="源版本" error={tried ? errors.src : undefined} hint="可以选任何线上的版本，包括其他维护线。">
          <select value={src} onChange={(e) => setSrc(e.target.value)} className="h-8 w-full rounded-sm border border-line bg-paper px-1.5">
            <option value="">选择版本</option>
            {lineageOrder(s, appKey).map(({ line }) => {
              const vs = lineVersions(s, line.id);
              if (!vs.length) return null;
              return (
                <optgroup key={line.id} label={`${line.isMain ? '主线' : line.name}${line.status === '终止' ? '（已终止）' : ''}`}>
                  {vs.slice().reverse().map((v) => (
                    <option key={v.id} value={v.id}>{v.number}　{v.releasedAt}</option>
                  ))}
                </optgroup>
              );
            })}
          </select>
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="名称" error={tried ? errors.name : undefined}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：ACME 定制" />
          </Field>
          <Field label="Git 分支名" error={tried ? errors.branch : undefined} hint="只登记，不会在仓库里创建分支">
            <Input value={branch} onChange={(e) => setBranch(e.target.value)} placeholder="cust/acme" className="font-mono" />
          </Field>
        </div>
        <Field label="分叉原因" error={tried ? errors.reason : undefined}>
          <Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} placeholder="例如：客户合同要求定制对账单格式，主线不接受" />
        </Field>
        <div className="grid grid-cols-2 gap-4">
          <Field label="计划终止日期" hint="临近时会出现在首页「即将到期的维护线」">
            <Input type="date" min={TODAY} value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
          </Field>
          <Field label="备注">
            <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
        </div>
      </div>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind="primary" onClick={submit}>分叉</Button>
      </DialogActions>
    </Dialog>
  );
}

export function VersionDialog({ lineId, onClose }: { lineId: string | null; onClose: () => void }) {
  const s = useStore();
  const line = lineId ? lineById(lineId) : undefined;
  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<'tag' | 'commit'>('tag');
  const [ref, setRef] = useState('');
  const [releasedAt, setReleasedAt] = useState(TODAY);
  const [note, setNote] = useState('');
  const [tried, setTried] = useState(false);
  if (!line) return null;

  const frozen = line.status === '冻结';
  const clash = appVersions(s, line.appKey).find((v) => v.number === number.trim());
  const clashLine = clash ? lineById(clash.lineId) : undefined;
  const errors = {
    number: !number.trim() ? '填写版本号' : clash ? `版本号在本应用内已存在（${clashLine?.isMain ? '主线' : clashLine?.name}，${clash.releasedAt}）` : undefined,
    ref: !ref.trim() ? `填写 ${kind}` : undefined,
    note: frozen && !note.trim() ? '冻结的维护线只允许紧急修复，请写明修复了什么' : undefined,
  };
  const ok = !Object.values(errors).some(Boolean);

  const submit = () => {
    setTried(true);
    if (!ok) return;
    actions.registerVersion({ lineId: line.id, number: number.trim(), ref: { kind, value: ref.trim() }, releasedAt, note: note.trim() });
    onClose();
  };

  const last = lineVersions(s, line.id).at(-1);
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} title={`在${line.isMain ? '主线' : `「${line.name}」`}登记版本`} description={<>分支 <Mono>{line.branch}</Mono>{last && <>，上一个版本 {last.number}</>}。人工登记，不校验 tag 或 commit 是否存在。</>}>
      <div className="space-y-4">
        <Field label="版本号" error={tried || clash ? errors.number : undefined} hint="格式不限，同一应用内唯一">
          <Input value={number} onChange={(e) => setNumber(e.target.value)} autoFocus />
        </Field>
        <Field label="对应的 tag 或 commit" error={tried ? errors.ref : undefined}>
          <div className="flex gap-2">
            <div className="flex shrink-0 border border-line text-[13px]">
              {(['tag', 'commit'] as const).map((k) => (
                <button key={k} type="button" onClick={() => setKind(k)} className={`px-3 ${kind === k ? 'bg-ink text-white' : 'bg-paper text-ink-2'}`}>{k}</button>
              ))}
            </div>
            <Input value={ref} onChange={(e) => setRef(e.target.value)} className="font-mono" placeholder={kind === 'tag' ? 'v2.3.3' : '9f03be1'} />
          </div>
        </Field>
        <Field label="发布时间">
          <Input type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} />
        </Field>
        <Field label={frozen ? '说明（冻结状态下必填）' : '说明'} error={tried ? errors.note : undefined}>
          <Textarea rows={2} value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
      </div>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button kind="primary" onClick={submit}>登记</Button>
      </DialogActions>
    </Dialog>
  );
}
