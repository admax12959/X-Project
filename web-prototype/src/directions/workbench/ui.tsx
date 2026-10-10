import { Dialog as BaseDialog } from '@base-ui/react/dialog';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react';
import { daysUntil, isExpiring, type IdentityType, type Line, type LineStatus } from '../../data';

export const IDENTITY_LABEL: Record<IdentityType, string> = { ldap: 'LDAP', local: '本地账号密码' };

type Kind = 'plain' | 'primary' | 'danger' | 'solid-danger';
export function Btn({ kind = 'plain', className = '', type = 'button', ...p }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: Kind }) {
  const k = kind === 'plain' ? '' : `wb-btn-${kind}`;
  return <button type={type} {...p} className={`wb-btn ${k} ${className}`} />;
}

export function LinkBtn({ className = '', ...p }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type="button" {...p} className={`wb-link text-left ${className}`} />;
}

export function Input({ invalid, className = '', ...p }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return <input {...p} aria-invalid={invalid || undefined} className={`wb-input ${className}`} />;
}
export function Textarea({ invalid, className = '', ...p }: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean }) {
  return <textarea {...p} aria-invalid={invalid || undefined} className={`wb-input ${className}`} />;
}
export function Select({ invalid, className = '', ...p }: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return <select {...p} aria-invalid={invalid || undefined} className={`wb-input ${className}`} />;
}

export function Field({ label, hint, error, children, className = '' }: { label: ReactNode; hint?: ReactNode; error?: string; children: ReactNode; className?: string }) {
  return (
    <label className={`block ${className}`}>
      <span className="wb-label">{label}</span>
      {children}
      {error ? <span className="wb-error">{error}</span> : hint ? <span className="wb-hint">{hint}</span> : null}
    </label>
  );
}

export function Mono({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <code className={`wb-mono ${className}`}>{children}</code>;
}

/**
 * The rail glyph: the same stroke language as the lineage graph, shrunk.
 * 活跃 = one heavy rail, 冻结 = a hollow double rail, 终止 = thin grey rail with an end bar.
 */
export function RailGlyph({ status, main = false, width = 18 }: { status: LineStatus; main?: boolean; width?: number }) {
  const h = 10;
  const y = h / 2;
  if (status === '终止')
    return (
      <svg width={width} height={h} aria-hidden className="shrink-0">
        <line x1={0} x2={width - 2} y1={y} y2={y} stroke="var(--wb-retired)" strokeWidth={1.5} />
        <line x1={width - 1.5} x2={width - 1.5} y1={1} y2={h - 1} stroke="var(--wb-retired)" strokeWidth={2} />
      </svg>
    );
  if (status === '冻结')
    return (
      <svg width={width} height={h} aria-hidden className="shrink-0">
        <line x1={0} x2={width} y1={y - 2} y2={y - 2} stroke="var(--color-ink)" strokeWidth={1.3} />
        <line x1={0} x2={width} y1={y + 2} y2={y + 2} stroke="var(--color-ink)" strokeWidth={1.3} />
      </svg>
    );
  return (
    <svg width={width} height={h} aria-hidden className="shrink-0">
      <line x1={0} x2={width} y1={y} y2={y} stroke="var(--color-ink)" strokeWidth={main ? 4 : 2.5} />
    </svg>
  );
}

export function dueText(line: Line): string | null {
  if (!line.plannedEnd || line.status === '终止') return null;
  const d = daysUntil(line.plannedEnd);
  return d < 0 ? `已过期 ${-d} 天` : d === 0 ? '今天到期' : `${d} 天后到期`;
}

export function PlannedEnd({ line }: { line: Line }) {
  if (!line.plannedEnd) return <span className="text-ink-3">未设定</span>;
  const due = dueText(line);
  return (
    <span>
      {line.plannedEnd}
      {isExpiring(line) && due && <span className="ml-2 font-semibold text-accent">{due}</span>}
    </span>
  );
}

export function Dialog({ open = true, onClose, title, description, children, footer, width = 520 }: { open?: boolean; onClose: () => void; title: ReactNode; description?: ReactNode; children: ReactNode; footer?: ReactNode; width?: number }) {
  return (
    <BaseDialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className="wb-backdrop" />
        <BaseDialog.Popup className="wb-dialog" style={{ width }}>
          <div className="border-b border-line-2 px-6 pt-5 pb-4">
            <BaseDialog.Title className="text-[length:var(--wb-text-lg)] leading-tight font-semibold">{title}</BaseDialog.Title>
            {description && <BaseDialog.Description className="mt-1.5 text-[length:var(--wb-text-sm)] leading-relaxed text-ink-2">{description}</BaseDialog.Description>}
          </div>
          <div className="px-6 py-5">{children}</div>
          {footer && <div className="flex items-center justify-end gap-2 border-t border-line-2 bg-[var(--wb-hover-row)] px-6 py-3">{footer}</div>}
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}

/** Definition rows for the inspector: label column fixed, value column wraps. */
export function Dl({ children }: { children: ReactNode }) {
  return <dl className="grid grid-cols-[76px_minmax(0,1fr)] gap-x-3 gap-y-2.5">{children}</dl>;
}
export function Dt({ children }: { children: ReactNode }) {
  return <dt className="pt-px text-[length:var(--wb-text-sm)] text-ink-3">{children}</dt>;
}
export function Dd({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <dd className={`min-w-0 leading-snug break-words ${className}`}>{children}</dd>;
}

export function PaneHeading({ children, actions }: { children: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-3 flex items-center justify-between gap-3">
      <h3 className="text-[length:var(--wb-text-md)] font-semibold">{children}</h3>
      {actions && <div className="flex gap-2">{actions}</div>}
    </div>
  );
}
