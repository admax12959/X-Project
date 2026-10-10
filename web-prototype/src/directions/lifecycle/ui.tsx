import { Dialog as BaseDialog } from '@base-ui/react/dialog';
import { cloneElement, isValidElement, useId, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactElement, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import { userName, type User } from '../../data';

type Kind = 'default' | 'primary' | 'ghost' | 'danger';
export function Button({ kind = 'default', className = '', type = 'button', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: Kind }) {
  const k = kind === 'default' ? '' : `lc-btn--${kind}`;
  return <button type={type} {...props} className={`lc-btn ${k} ${className}`} />;
}

export function Input({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`lc-input ${className}`} />;
}
export function Textarea({ className = '', ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`lc-input ${className}`} />;
}
export function Select({ className = '', ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={`lc-input ${className}`} />;
}

/**
 * Label above, helper below, error replaces helper in the same slot (no jump).
 * The single child control gets id, aria-invalid and aria-describedby wired in.
 */
export function Field({ label, help, error, children, className = '' }: { label: ReactNode; help?: ReactNode; error?: string; children: ReactNode; className?: string }) {
  const id = useId();
  const helpId = `${id}-help`;
  const control = isValidElement(children)
    ? cloneElement(children as ReactElement<Record<string, unknown>>, { id, 'aria-invalid': error ? true : undefined, 'aria-describedby': helpId })
    : children;
  return (
    <div className={className}>
      <label htmlFor={id} className="lc-field-label">{label}</label>
      {control}
      <span id={helpId} className="lc-field-help" data-error={error ? '' : undefined}>
        {error ?? help}
      </span>
    </div>
  );
}

/** Read-only fact in a form: same rhythm as a Field, no control. */
export function Fact({ label, children, help }: { label: ReactNode; children: ReactNode; help?: ReactNode }) {
  return (
    <div>
      <div className="lc-field-label">{label}</div>
      <div className="flex min-h-9 items-center border-b border-[var(--lc-rule)]">{children}</div>
      <span className="lc-field-help">{help}</span>
    </div>
  );
}

export function Code({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <span className={`lc-code ${className}`}>{children}</span>;
}

export function Dialog({ open = true, onClose, title, description, children, width = 520 }: { open?: boolean; onClose: () => void; title: ReactNode; description?: ReactNode; children: ReactNode; width?: number }) {
  return (
    <BaseDialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className="lc-scrim" />
        <BaseDialog.Popup className="lc-dialog" style={{ width }}>
          <BaseDialog.Title className="text-[length:var(--lc-text-lg)] leading-tight font-bold">{title}</BaseDialog.Title>
          {description && <BaseDialog.Description className="mt-2 max-w-[60ch] text-[length:var(--lc-text-sm)] leading-relaxed text-[var(--lc-ink-2)]">{description}</BaseDialog.Description>}
          <div className="mt-6">{children}</div>
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}

export function DialogActions({ children, left }: { children: ReactNode; left?: ReactNode }) {
  return (
    <div className="mt-6 flex items-center gap-2 border-t border-[var(--lc-rule)] pt-4">
      <div className="mr-auto flex items-center gap-2">{left}</div>
      {children}
    </div>
  );
}

export function Seg<T extends string>({ value, options, onChange, label }: { value: T; options: { value: T; label: string }[]; onChange: (v: T) => void; label: string }) {
  return (
    <div className="lc-seg" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.value} type="button" aria-pressed={value === o.value} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

/** Pick users for Owner / PE: selected names in a ruled line, add from 正常 users only. */
export function UserPicker({ label, value, onChange, users, error, help }: { label: string; value: string[]; onChange: (v: string[]) => void; users: User[]; error?: string; help?: ReactNode }) {
  const options = users.filter((u) => u.status === '正常' && !value.includes(u.username));
  return (
    <Field label={label} error={error} help={help}>
      <div className="flex min-h-9 flex-wrap items-center gap-x-1 gap-y-1 border border-[var(--lc-edge)] px-1 py-1" role="group" aria-label={label}>
        {value.map((u) => {
          const off = users.find((x) => x.username === u)?.status !== '正常';
          return (
            <span key={u} className={`inline-flex h-7 items-center gap-1 bg-[var(--lc-paper-3)] pl-2 text-[length:var(--lc-text-sm)] ${off ? 'text-[var(--lc-error)]' : ''}`}>
              {userName(u)}
              {off && '（已禁用）'}
              <button type="button" aria-label={`移除 ${label} ${userName(u)}`} onClick={() => onChange(value.filter((x) => x !== u))} className="lc-row h-7 w-7 text-[var(--lc-muted)]">
                ×
              </button>
            </span>
          );
        })}
        <select
          aria-label={`添加 ${label}`}
          value=""
          disabled={options.length === 0}
          onChange={(e) => e.target.value && onChange([...value, e.target.value])}
          className="lc-input !h-7 !w-auto !border-transparent !px-1 text-[length:var(--lc-text-sm)]"
        >
          <option value="">{options.length ? `添加 ${label}…` : '没有可添加的用户'}</option>
          {options.map((u) => (
            <option key={u.username} value={u.username}>
              {u.displayName}（{u.username}）
            </option>
          ))}
        </select>
      </div>
    </Field>
  );
}

/** Mini legend swatch for the lifecycle encodings. */
export function Swatch({ className }: { className: string }) {
  return <span aria-hidden className={`inline-block h-2.5 w-7 align-middle ${className}`} />;
}
