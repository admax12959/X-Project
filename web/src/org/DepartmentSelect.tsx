// A <select> of the department tree: every department after its parent, indented by depth.
import { useEffect, useState } from 'react';
import { api } from '../api/client.ts';
import type { Department } from '../api/types.ts';
import { Select } from '../ui/controls.tsx';
import { treeRows, type DepartmentNode } from './tree.ts';

const INDENT = '    ';

/** Loads the department tree once; null while loading or when it cannot be read. */
export function useDepartments(): Department[] | null {
  const [items, setItems] = useState<Department[] | null>(null);
  useEffect(() => {
    let live = true;
    api.departments.list().then(
      (r) => live && setItems(r.items),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);
  return items;
}

export function DepartmentSelect({
  departments,
  value,
  onChange,
  noneLabel,
  excluded,
  ...rest
}: {
  departments: DepartmentNode[];
  /** Department ID, or '' for the empty choice. */
  value: string;
  onChange: (id: string) => void;
  /** Label of the empty choice ("no department", "all departments", "top level"). */
  noneLabel: string;
  /** Departments that cannot be chosen and are left out, with nothing below them. */
  excluded?: Set<string>;
  'aria-label'?: string;
  name?: string;
  className?: string;
  disabled?: boolean;
}) {
  const rows = treeRows(departments.filter((d) => !excluded?.has(d.id)));
  return (
    <Select {...rest} value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">{noneLabel}</option>
      {rows.map(({ dept, depth }) => (
        <option key={dept.id} value={dept.id}>
          {INDENT.repeat(depth)}
          {dept.name}
        </option>
      ))}
    </Select>
  );
}
