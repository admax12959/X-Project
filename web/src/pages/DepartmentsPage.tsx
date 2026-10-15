// Administration > Departments (#25): the department tree. New, rename, move, delete. Administrators only.
import { Dialog } from '@base-ui/react/dialog';
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { Department } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { DepartmentSelect } from '../org/DepartmentSelect.tsx';
import { descendantIds, pathLabel, treeRows } from '../org/tree.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty, Field, Input } from '../ui/controls.tsx';
import { th, td } from '../ui/table.ts';

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: Department[] };
// create = a new department under `parentId`; edit = rename / move `dept`; remove = confirm deleting `dept`.
type Dialog = null | { kind: 'create'; parentId: string } | { kind: 'edit'; dept: Department } | { kind: 'remove'; dept: Department };

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function DepartmentsPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <Departments />;
}

function Departments() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [dialog, setDialog] = useState<Dialog>(null);

  const reload = useCallback(() => {
    api.departments.list().then(
      (r) => setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  const items = load.status === 'ok' ? load.items : [];
  const rows = treeRows(items);
  const done = () => {
    setDialog(null);
    reload();
  };

  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-4">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('departments.title')}
          {load.status === 'ok' && <span className="font-normal text-text-3">({items.length})</span>}
        </h1>
        <Button kind="primary" onClick={() => setDialog({ kind: 'create', parentId: '' })}>
          {t('departments.create')}
        </Button>
      </div>
      <p className="text-text-2">{t('departments.intro')}</p>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('departments.col.name')}</th>
              <th className={th}>{t('departments.col.members')}</th>
              <th className={th}>{t('departments.col.children')}</th>
              <th className={th}>{t('departments.col.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(({ dept, depth }) => (
              <tr key={dept.id} className="hover:bg-hover">
                <td className={td}>
                  <span style={{ paddingLeft: `${depth * 20}px` }}>{dept.name}</span>
                </td>
                <td className={td}>{dept.memberCount}</td>
                <td className={td}>{dept.childCount}</td>
                <td className={`${td} whitespace-nowrap`}>
                  <span className="inline-flex gap-2">
                    <Button small onClick={() => setDialog({ kind: 'create', parentId: dept.id })}>
                      {t('departments.addChild')}
                    </Button>
                    <Button small onClick={() => setDialog({ kind: 'edit', dept })}>
                      {t('departments.edit')}
                    </Button>
                    <Button small onClick={() => setDialog({ kind: 'remove', dept })}>
                      {t('departments.remove')}
                    </Button>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('departments.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && <Empty title={t('departments.empty')} />}
      </div>

      {dialog?.kind === 'create' && <EditDialog departments={items} initialParent={dialog.parentId} onClose={() => setDialog(null)} onSaved={done} />}
      {dialog?.kind === 'edit' && <EditDialog departments={items} dept={dialog.dept} onClose={() => setDialog(null)} onSaved={done} />}
      {dialog?.kind === 'remove' && <RemoveDialog departments={items} dept={dialog.dept} onClose={() => setDialog(null)} onDeleted={done} />}
    </>
  );
}

function Frame({ title, onClose, onSubmit, children }: { title: string; onClose: () => void; onSubmit: (e: FormEvent) => void; children: ReactNode }) {
  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[80vh] w-[440px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-lg font-bold">{title}</Dialog.Title>
          <form onSubmit={onSubmit} noValidate className="grid gap-3.5">
            {children}
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** Create (no `dept`) or rename / move (`dept`) a department. */
function EditDialog({
  departments,
  dept,
  initialParent = '',
  onClose,
  onSaved,
}: {
  departments: Department[];
  dept?: Department;
  initialParent?: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useT();
  const [name, setName] = useState(dept?.name ?? '');
  const [parentId, setParentId] = useState(dept ? (dept.parentId ?? '') : initialParent);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  // A department cannot move under itself or below itself.
  const excluded = dept ? descendantIds(departments, dept.id) : undefined;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (dept) {
        const changes: { name?: string; parentId?: string | null } = {};
        if (name !== dept.name) changes.name = name;
        if (parentId !== (dept.parentId ?? '')) changes.parentId = parentId || null;
        if (Object.keys(changes).length > 0) await api.departments.update(dept.id, changes);
      } else {
        await api.departments.create({ name, parentId: parentId || null });
      }
      onSaved();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Frame title={dept ? t('departments.editTitle', { name: dept.name }) : t('departments.createTitle')} onClose={onClose} onSubmit={submit}>
      {errors[''] && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errors['']}
        </p>
      )}
      <Field label={t('departments.field.name')} error={errors.name}>
        <Input name="name" autoFocus autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label={t('departments.field.parent')} error={errors.parentId}>
        <DepartmentSelect name="parentId" departments={departments} value={parentId} onChange={setParentId} noneLabel={t('departments.topLevel')} excluded={excluded} />
      </Field>
      <div className="mt-2 flex justify-end gap-2">
        <Button type="button" onClick={onClose}>
          {t('departments.cancel')}
        </Button>
        <Button kind="primary" type="submit" disabled={busy}>
          {busy ? t('departments.saving') : t('departments.save')}
        </Button>
      </div>
    </Frame>
  );
}

function RemoveDialog({ departments, dept, onClose, onDeleted }: { departments: Department[]; dept: Department; onClose: () => void; onDeleted: () => void }) {
  const { t } = useT();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.departments.remove(dept.id);
      onDeleted();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };
  return (
    <Frame title={t('departments.removeTitle', { name: dept.name })} onClose={onClose} onSubmit={submit}>
      {error !== null && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errorsByField(error)['']}
        </p>
      )}
      <p>{t('departments.removeConfirm', { path: pathLabel(departments, dept.id) })}</p>
      <div className="mt-2 flex justify-end gap-2">
        <Button type="button" onClick={onClose}>
          {t('departments.cancel')}
        </Button>
        <Button kind="danger" type="submit" disabled={busy}>
          {busy ? t('departments.removing') : t('departments.remove')}
        </Button>
      </div>
    </Frame>
  );
}
