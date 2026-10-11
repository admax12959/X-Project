import { Dialog as BaseDialog } from '@base-ui/react/dialog';
import { CircleCheck, CircleMinus, CirclePause } from 'lucide-react';
import { useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import type { LineStatus, Line } from '../../data';
import { useT } from '../../i18n';
import type { Msg } from '../../i18n';

type Kind = 'plain' | 'primary' | 'danger';
export function Btn({ kind = 'plain', small, className = '', type = 'button', ...p }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: Kind; small?: boolean }) {
  return <button type={type} {...p} className={`cs-btn ${kind === 'primary' ? 'cs-btn-primary' : kind === 'danger' ? 'cs-btn-danger' : ''} ${small ? 'cs-btn-sm' : ''} ${className}`} />;
}
export function Cmd({ icon, children, ...p }: ButtonHTMLAttributes<HTMLButtonElement> & { icon: ReactNode }) {
  return (
    <button type="button" {...p} className="cs-cmd">
      {icon}
      {children}
    </button>
  );
}
export function LinkBtn({ className = '', ...p }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type="button" {...p} className={`cs-link ${className}`} />;
}
export function Mono({ children, className = '', title }: { children: ReactNode; className?: string; title?: string }) {
  return (
    <span className={`cs-mono ${className}`} title={title}>
      {children}
    </span>
  );
}

export function Input({ invalid, className = '', ...p }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return <input {...p} aria-invalid={invalid || undefined} className={`cs-input ${className}`} />;
}
export function Textarea({ invalid, className = '', ...p }: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean }) {
  return <textarea {...p} aria-invalid={invalid || undefined} className={`cs-textarea ${className}`} />;
}
export function Select({ invalid, className = '', ...p }: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return <select {...p} aria-invalid={invalid || undefined} className={`cs-select ${className}`} />;
}
export function Field({ label, hint, error, children, className = '' }: { label: ReactNode; hint?: ReactNode; error?: string; children: ReactNode; className?: string }) {
  return (
    <label className={`block ${className}`} style={{ display: 'block' }}>
      <span className="cs-label">{label}</span>
      {children}
      {error ? <span className="cs-error">{error}</span> : hint ? <span className="cs-hint">{hint}</span> : null}
    </label>
  );
}

const STATUS_ICON = { Active: CircleCheck, Frozen: CirclePause, Retired: CircleMinus } as const;
export function StatusText({ status, title }: { status: LineStatus; title?: string }) {
  const { t } = useT();
  const I = STATUS_ICON[status];
  return (
    <span className={`cs-status cs-status-${status}`} title={title}>
      <I size={16} aria-hidden />
      {t(`status.${status}`)}
    </span>
  );
}

export function UserStatusText({ status }: { status: 'Enabled' | 'Disabled' }) {
  const { t } = useT();
  const I = status === 'Enabled' ? CircleCheck : CircleMinus;
  return (
    <span className={`cs-status ${status === 'Enabled' ? 'cs-status-Active' : 'cs-userstatus-Disabled'}`}>
      <I size={16} aria-hidden />
      {t(`userStatus.${status}`)}
    </span>
  );
}

/** Display name of a line: seeded Mainline text, or the line's own name. */
export function useLineName() {
  const { tx, t } = useT();
  return (l: Line) => (l.isMain ? t('term.mainline') : tx(l.name));
}

export function Container({ title, actions, children, flush, className = '' }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; flush?: boolean; className?: string }) {
  return (
    <section className={`cs-container ${flush ? 'flush' : ''} ${className}`}>
      {(title || actions) && (
        <div className="cs-container-head">
          <h2 className="cs-h2">{title}</h2>
          {actions && <div className="cs-actions">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}

export function KV({ rows, single }: { rows: { label: ReactNode; value: ReactNode }[]; single?: boolean }) {
  return (
    <dl className={`cs-kv ${single ? 'cs-kv-1' : ''}`}>
      {rows.map((r, i) => (
        <div key={i} className="cs-kv-row">
          <dt>{r.label}</dt>
          <dd>{r.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Empty({ title, children }: { title: ReactNode; children?: ReactNode }) {
  return (
    <div className="cs-empty">
      <b>{title}</b>
      {children}
    </div>
  );
}

export function Dialog({ onClose, title, description, children, footer, width = 520 }: { onClose: () => void; title: ReactNode; description?: ReactNode; children?: ReactNode; footer?: ReactNode; width?: number }) {
  return (
    <BaseDialog.Root open onOpenChange={(o) => !o && onClose()}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className="cs-backdrop" />
        <BaseDialog.Popup className="cs-dialog" style={{ width }}>
          <div className="cs-dialog-head">
            <BaseDialog.Title className="cs-dialog-title">{title}</BaseDialog.Title>
            {description && <BaseDialog.Description className="cs-dialog-desc">{description}</BaseDialog.Description>}
          </div>
          {children && <div className="cs-dialog-body">{children}</div>}
          {footer && <div className="cs-dialog-foot">{footer}</div>}
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}

/** Copy to clipboard without ever throwing (clipboard can be denied). */
export function CopyButton({ value }: { value: string }) {
  const { t } = useT();
  const [done, setDone] = useState(false);
  return (
    <button
      type="button"
      className="cs-link cs-small"
      onClick={() => {
        Promise.resolve()
          .then(() => navigator.clipboard.writeText(value))
          .catch(() => undefined);
        setDone(true);
        setTimeout(() => setDone(false), 1500);
      }}
    >
      {done ? t('common.copied') : t('common.copy')}
    </button>
  );
}

export const msgText = (tm: (m: Msg) => string, m?: Msg) => (m ? tm(m) : undefined);
