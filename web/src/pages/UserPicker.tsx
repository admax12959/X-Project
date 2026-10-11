// Multi-select of users for the Owner / PE fields: filter box + checkbox list.
import { useState, type ReactNode } from 'react';
import type { AdminUser } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Field, Input } from '../ui/controls.tsx';

export function UserPicker({
  label,
  hint,
  users,
  selected,
  onChange,
  error,
}: {
  label: ReactNode;
  hint: ReactNode;
  /** Already limited to enabled users by the caller. */
  users: AdminUser[];
  selected: string[];
  onChange: (ids: string[]) => void;
  error?: string;
}) {
  const { t } = useT();
  const [filter, setFilter] = useState('');
  const needle = filter.trim().toLowerCase();
  const shown = users.filter((u) => !needle || `${u.displayName} ${u.username}`.toLowerCase().includes(needle));
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
        <ul className="max-h-36 overflow-y-auto py-1">
          {shown.map((u) => (
            <li key={u.id}>
              <label className="flex cursor-pointer items-center gap-2 px-2.5 py-1 hover:bg-subtle">
                <input type="checkbox" checked={selected.includes(u.id)} onChange={() => toggle(u.id)} />
                <span>{u.displayName}</span>
                <span className="font-mono text-[12.5px] text-text-3">{u.username}</span>
              </label>
            </li>
          ))}
          {shown.length === 0 && <li className="px-2.5 py-1 text-text-3">{t('apps.picker.empty')}</li>}
        </ul>
      </div>
      <p className="mt-1 text-[13px] text-text-3">{hint}</p>
    </Field>
  );
}
