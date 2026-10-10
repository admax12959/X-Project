import { Dialog as BaseDialog } from '@base-ui/react/dialog';
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, TextareaHTMLAttributes } from 'react';
import { daysUntil, isExpiring, type Line } from '../../data';

export function Button({ kind = 'default', className = '', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: 'default' | 'primary' | 'quiet' | 'danger' }) {
  const look = {
    default: 'border border-line bg-paper hover:bg-wash',
    primary: 'border border-accent bg-accent text-white hover:bg-[#244d66]',
    quiet: 'text-accent hover:underline px-0',
    danger: 'border border-line bg-paper text-danger hover:bg-wash',
  }[kind];
  return <button {...props} className={`h-8 rounded-sm px-3 text-[13px] transition-colors disabled:opacity-45 ${look} ${className}`} />;
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`h-8 w-full rounded-sm border border-line bg-paper px-2 outline-none focus:border-accent read-only:bg-wash read-only:text-ink-2 ${props.className ?? ''}`} />;
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`w-full rounded-sm border border-line bg-paper px-2 py-1.5 outline-none focus:border-accent ${props.className ?? ''}`} />;
}

export function Field({ label, hint, error, children }: { label: string; hint?: ReactNode; error?: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[13px] text-ink-2">{label}</span>
      {children}
      {error ? <span className="mt-1 block text-[12px] text-danger">{error}</span> : hint ? <span className="mt-1 block text-[12px] text-ink-3">{hint}</span> : null}
    </label>
  );
}

export function Mono({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <code className={`font-mono text-[12.5px] ${className}`}>{children}</code>;
}

/** Status in words. Only 终止 changes tone (it is read-only); nothing gets a traffic-light colour. */
export function StatusText({ line }: { line: Line }) {
  if (line.isMain) return null;
  const tone = line.status === '终止' ? 'text-ink-3' : line.status === '冻结' ? 'text-ink-2' : 'text-ink';
  return <span className={tone}>{line.status}</span>;
}

export function ExpiryText({ line }: { line: Line }) {
  if (!line.plannedEnd) return <span className="text-ink-3">—</span>;
  if (!isExpiring(line)) return <span className="text-ink-2">{line.plannedEnd}</span>;
  const d = daysUntil(line.plannedEnd);
  return (
    <span>
      {line.plannedEnd}
      <span className="ml-1.5 font-semibold text-danger">{d < 0 ? `已过期 ${-d} 天` : d === 0 ? '今天到期' : `${d} 天后到期`}</span>
    </span>
  );
}

export function Dialog({ open, onOpenChange, title, description, children, width = 480 }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; description?: ReactNode; children: ReactNode; width?: number }) {
  return (
    <BaseDialog.Root open={open} onOpenChange={onOpenChange}>
      <BaseDialog.Portal>
        <BaseDialog.Backdrop className="fixed inset-0 bg-black/25" />
        <BaseDialog.Popup style={{ width }} className="fixed top-[12vh] left-1/2 max-h-[76vh] -translate-x-1/2 overflow-y-auto rounded-sm border border-line bg-paper p-6 shadow-[0_2px_6px_rgba(0,0,0,0.12)] outline-none">
          <BaseDialog.Title className="text-[17px] font-semibold">{title}</BaseDialog.Title>
          {description && <BaseDialog.Description className="mt-1 text-[13px] text-ink-2">{description}</BaseDialog.Description>}
          <div className="mt-5">{children}</div>
        </BaseDialog.Popup>
      </BaseDialog.Portal>
    </BaseDialog.Root>
  );
}

export function DialogActions({ children }: { children: ReactNode }) {
  return <div className="mt-6 flex justify-end gap-2">{children}</div>;
}

export function PageHeader({ title, sub, actions }: { title: ReactNode; sub?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-6 flex items-end justify-between gap-6">
      <div>
        <h1 className="text-[24px] font-semibold leading-tight">{title}</h1>
        {sub && <div className="mt-1.5 text-ink-2">{sub}</div>}
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </div>
  );
}

export function SectionTitle({ children, actions }: { children: ReactNode; actions?: ReactNode }) {
  return (
    <div className="mb-3 flex items-baseline justify-between">
      <h2 className="text-[18px] font-semibold">{children}</h2>
      {actions}
    </div>
  );
}

/** Plain table: border lives on the table box itself, rows divided by hairlines. */
export function Table({ head, children }: { head: ReactNode[]; children: ReactNode }) {
  return (
    <table className="w-full border-collapse border border-line bg-paper text-left">
      <thead>
        <tr className="border-b border-line text-[12.5px] text-ink-2">
          {head.map((h, i) => (
            <th key={i} className="px-3 py-2 font-normal">{h}</th>
          ))}
        </tr>
      </thead>
      <tbody className="[&>tr]:border-b [&>tr]:border-line-2 [&_td]:px-3 [&_td]:py-2.5 [&_td]:align-top">{children}</tbody>
    </table>
  );
}
