import { Dialog as BaseDialog } from '@base-ui/react/dialog';
import { useId, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import { daysUntil, isExpiring, userName, type Line, type User } from '../../data';

export function Button({ kind = 'default', size, className = '', type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: 'default' | 'primary' | 'danger'; size?: 'sm' }) {
  const k = kind === 'primary' ? 'rl-btn--primary' : kind === 'danger' ? 'rl-btn--danger' : '';
  return <button type={type} {...props} className={`rl-btn ${k} ${size === 'sm' ? 'rl-btn--sm' : ''} ${className}`} />;
}

export function LinkButton({ className = '', type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type={type} {...props} className={`rl-link ${className}`} />;
}

export function Mono({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <code className={`rl-mono ${className}`}>{children}</code>;
}

/** Label above, helper/error below in a slot that never collapses. */
export function Field({ label, hint, error, children, className = '' }: { label: ReactNode; hint?: ReactNode; error?: string; children: (a: { id: string; describedBy: string; invalid: boolean }) => ReactNode; className?: string }) {
  const id = useId();
  const helpId = `${id}-help`;
  return (
    <div className={`mb-2 ${className}`}>
      <label htmlFor={id} className="mb-2 block leading-5 font-medium rl-ink">{label}</label>
      {children({ id, describedBy: helpId, invalid: !!error })}
      <div id={helpId} className="rl-help" data-error={error ? '' : undefined} aria-live="polite">{error ?? hint}</div>
    </div>
  );
}

type F = { f: { id: string; describedBy: string; invalid: boolean } };
export function Input({ f, className = '', ...p }: InputHTMLAttributes<HTMLInputElement> & F) {
  return <input id={f.id} aria-describedby={f.describedBy} aria-invalid={f.invalid || undefined} {...p} className={`rl-input ${className}`} />;
}
export function Textarea({ f, className = '', ...p }: TextareaHTMLAttributes<HTMLTextAreaElement> & F) {
  return <textarea id={f.id} aria-describedby={f.describedBy} aria-invalid={f.invalid || undefined} {...p} className={`rl-input ${className}`} />;
}
export function Select({ f, className = '', ...p }: SelectHTMLAttributes<HTMLSelectElement> & F) {
  return <select id={f.id} aria-describedby={f.describedBy} aria-invalid={f.invalid || undefined} {...p} className={`rl-input ${className}`} />;
}

export function Segmented<T extends string>({ name, value, options, onChange, label }: { name: string; value: T; options: { value: T; label: string }[]; onChange: (v: T) => void; label: string }) {
  return (
    <div role="radiogroup" aria-label={label} className="rl-seg">
      {options.map((o) => (
        <label key={o.value}>
          <input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />
          {o.label}
        </label>
      ))}
    </div>
  );
}

export function Dialog({ open = true, onClose, title, description, children, footer, width = 520 }: { open?: boolean; onClose: () => void; title: ReactNode; description?: ReactNode; children?: ReactNode; footer: ReactNode; width?: number }) {
  return (
    <BaseDialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className="rl-scrim" />
        <BaseDialog.Popup className="rl-dialog" style={{ width }}>
          <div className="min-h-0 overflow-y-auto px-6 pt-5 pb-6">
            <BaseDialog.Title className="rl-t-lg">{title}</BaseDialog.Title>
            {description && <BaseDialog.Description className="mt-1 rl-ink-2 rl-prose">{description}</BaseDialog.Description>}
            {children && <div className="mt-5">{children}</div>}
          </div>
          <div className="flex shrink-0 items-center justify-end gap-2 rl-rule-t rl-bg-2 px-6 py-3">{footer}</div>
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}

export const STATUS_MEANING: Record<Line['status'], string> = {
  活跃: '可以正常发版',
  冻结: '只允许紧急修复',
  终止: '不再发版，只读',
};

export function expiryWords(line: Line): string | null {
  if (!line.plannedEnd || !isExpiring(line)) return null;
  const d = daysUntil(line.plannedEnd);
  return d < 0 ? `已过期 ${-d} 天` : d === 0 ? '今天到期' : `${d} 天后到期`;
}

/** Status as a word; 活跃 in green ink (it is the "alive" meaning), the rest neutral. */
export function StatusWord({ line, stamp = false }: { line: Line; stamp?: boolean }) {
  const tone = line.status === '活跃' ? 'rl-green' : line.status === '冻结' ? 'rl-ink' : 'rl-ink-3';
  return <span className={`${stamp ? 'rl-stamp' : 'font-semibold'} ${tone}`}>{line.status}</span>;
}

/** Owner / PE picker: chips of current picks + a native select to add one. Only 正常 users can be added. */
export function PeoplePicker({ f, value, onChange, users, exclude = [], addLabel, minOne = false }: F & { value: string[]; onChange: (v: string[]) => void; users: User[]; exclude?: string[]; addLabel: string; minOne?: boolean }) {
  const candidates = users.filter((u) => u.status === '正常' && !value.includes(u.username) && !exclude.includes(u.username));
  return (
    <div className="flex flex-wrap items-center gap-2">
      {value.map((u) => (
        <span key={u} className="rl-chip">
          {userName(u)}
          <button type="button" aria-label={`移除 ${userName(u)}`} disabled={minOne && value.length === 1} title={minOne && value.length === 1 ? '至少保留一个 Owner' : undefined} onClick={() => onChange(value.filter((x) => x !== u))}>
            ×
          </button>
        </span>
      ))}
      <select
        id={f.id}
        aria-describedby={f.describedBy}
        aria-invalid={f.invalid || undefined}
        className="rl-input"
        style={{ width: 'auto' }}
        value=""
        disabled={candidates.length === 0}
        onChange={(e) => e.target.value && onChange([...value, e.target.value])}
      >
        <option value="">{candidates.length ? addLabel : '没有可添加的用户'}</option>
        {candidates.map((u) => (
          <option key={u.username} value={u.username}>
            {u.displayName}（{u.username}）
          </option>
        ))}
      </select>
    </div>
  );
}

export function Empty({ children }: { children: ReactNode }) {
  return <p className="py-4 rl-ink-3">{children}</p>;
}
