// Multi-select of users for the Owner / PE fields: filter box + checkbox list.
import { useEffect, useState, type ReactNode } from 'react';
import { api } from '../api/client.ts';
import { useT } from '../i18n/index.ts';
import { DepartmentSelect, useDepartments } from '../org/DepartmentSelect.tsx';
import { Avatar } from '../ui/Avatar.tsx';
import { Field, Input } from '../ui/controls.tsx';

export type PickerUser = { id: string; username: string; displayName: string; avatarUrl?: string | null; disabled?: boolean };

/** The enabled users in a department or below (any logged-in user may ask). */
const assignableInDepartment = (departmentId: string): Promise<PickerUser[]> => api.assignableUsers.list(departmentId).then((r) => r.items);

export function UserPicker({
  label,
  hint,
  users,
  selected,
  onChange,
  error,
  loadInDepartment = assignableInDepartment,
}: {
  label: ReactNode;
  hint: ReactNode;
  /** Enabled users, plus (when editing) people who are already selected but have been disabled: those carry `disabled`. */
  users: PickerUser[];
  selected: string[];
  onChange: (ids: string[]) => void;
  error?: string;
  /** The enabled users in a department or below, for the department filter (offered when the organization has departments). */
  loadInDepartment?: (departmentId: string) => Promise<PickerUser[]>;
}) {
  const { t } = useT();
  const [filter, setFilter] = useState('');
  const departments = useDepartments();
  const [departmentId, setDepartmentId] = useState('');
  // The users of the chosen department; null while loading or when no department is chosen.
  const [inDepartment, setInDepartment] = useState<PickerUser[] | null>(null);
  const [departmentError, setDepartmentError] = useState(false);
  useEffect(() => {
    setInDepartment(null);
    setDepartmentError(false);
    if (!departmentId) return;
    let live = true;
    loadInDepartment(departmentId).then(
      (r) => live && setInDepartment(r),
      () => live && setDepartmentError(true),
    );
    return () => {
      live = false;
    };
  }, [departmentId, loadInDepartment]);
  const needle = filter.trim().toLowerCase();
  const pool = departmentId ? (inDepartment ?? []) : users;
  const shown = pool.filter((u) => !needle || `${u.displayName} ${u.username}`.toLowerCase().includes(needle));
  const toggle = (id: string) => onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);

  return (
    <Field label={label} error={error}>
      <div className="rounded border border-line">
        <Input
          aria-label={t('apps.picker.filter')}
          placeholder={t('apps.picker.filter')}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="rounded-none border-0 border-b"
        />
        {departments !== null && departments.length > 0 && (
          <DepartmentSelect
            aria-label={t('apps.picker.department')}
            className="rounded-none border-0 border-b"
            departments={departments}
            value={departmentId}
            onChange={setDepartmentId}
            noneLabel={t('apps.picker.allDepartments')}
          />
        )}
        <ul className="max-h-36 overflow-y-auto py-1">
          {shown.map((u) => (
            <li key={u.id}>
              <label className="flex cursor-pointer items-center gap-2 px-2.5 py-1 hover:bg-subtle">
                <input type="checkbox" checked={selected.includes(u.id)} onChange={() => toggle(u.id)} />
                <Avatar size="sm" name={u.displayName} url={u.avatarUrl} />
                <span>{u.displayName}</span>
                <span className="font-mono text-[12.5px] text-text-3">{u.username}</span>
                {u.disabled && <span className="text-text-3">{t('apps.disabledSuffix')}</span>}
              </label>
            </li>
          ))}
          {departmentError && <li className="px-2.5 py-1 text-danger">{t('apps.picker.loadFailed')}</li>}
          {shown.length === 0 && !departmentError && <li className="px-2.5 py-1 text-text-3">{t('apps.picker.empty')}</li>}
        </ul>
      </div>
      <p className="mt-1 text-[13px] text-text-3">{hint}</p>
    </Field>
  );
}
