// Shared controls built on Base UI, styled with the Console tokens.
import { Button as BaseButton } from '@base-ui/react/button';
import { Field as BaseField } from '@base-ui/react/field';
import { Input as BaseInput } from '@base-ui/react/input';
import { ToggleGroup } from '@base-ui/react/toggle-group';
import { Toggle } from '@base-ui/react/toggle';
import type { ComponentProps, ReactNode } from 'react';
import { fill } from './width.ts';

const btnBase = 'inline-flex h-8 items-center justify-center gap-1.5 whitespace-nowrap rounded border px-3.5 font-semibold disabled:cursor-not-allowed';
const btnKinds = {
  plain: 'border-line bg-white text-text hover:bg-subtle disabled:border-sep disabled:text-text-3',
  primary: 'border-accent bg-accent text-white hover:border-accent-hover hover:bg-accent-hover disabled:border-disabled disabled:bg-disabled',
  danger: 'border-danger bg-danger text-white hover:bg-danger-hover',
} as const;

export function Button({ kind = 'plain', small, className = '', ...p }: ComponentProps<typeof BaseButton> & { kind?: keyof typeof btnKinds; small?: boolean }) {
  return <BaseButton {...p} className={`${btnBase} ${btnKinds[kind]} ${small ? 'h-7 px-2.5 text-[13px]' : ''} ${className}`} />;
}

/** Label + control + error, wired for accessibility by Base UI Field. */
export function Field({ label, error, children, className = '' }: { label: ReactNode; error?: string; children: ReactNode; className?: string }) {
  return (
    <BaseField.Root invalid={!!error} className={`block ${className}`}>
      <BaseField.Label className="mb-1 block text-[13px] font-semibold">{label}</BaseField.Label>
      {children}
      <BaseField.Error match={!!error} className="mt-1 block text-[13px] text-danger">
        {error}
      </BaseField.Error>
    </BaseField.Root>
  );
}

export function Input({ className = '', ...p }: ComponentProps<typeof BaseInput>) {
  return (
    <BaseInput
      {...p}
      className={`h-8 ${fill(className)} min-w-0 rounded border border-line bg-white px-2.5 focus:border-accent focus:outline-2 focus:-outline-offset-1 focus:outline-accent data-[invalid]:border-danger ${className}`}
    />
  );
}

/** Single-choice segmented control. */
export function Segmented<T extends string>({ value, options, onChange, label }: { value: T; options: { value: T; label: ReactNode }[]; onChange: (v: T) => void; label: string }) {
  return (
    <ToggleGroup
      aria-label={label}
      value={[value]}
      onValueChange={(v) => v[0] && onChange(v[0] as T)}
      className="inline-flex overflow-hidden rounded border border-line"
    >
      {options.map((o) => (
        <Toggle
          key={o.value}
          value={o.value}
          className="h-7 border-r border-line bg-white px-3 text-[13px] last:border-r-0 data-[pressed]:bg-sel data-[pressed]:font-semibold data-[pressed]:text-accent"
        >
          {o.label}
        </Toggle>
      ))}
    </ToggleGroup>
  );
}

const fieldBox =
  'min-w-0 rounded border border-line bg-white px-2.5 focus:border-accent focus:outline-2 focus:-outline-offset-1 focus:outline-accent';

export function Textarea({ className = '', ...p }: ComponentProps<'textarea'>) {
  return <textarea {...p} className={`${fieldBox} ${fill(className)} min-h-20 py-1.5 ${className}`} />;
}

/** Native select, styled like Input (a short, fixed list of choices). */
export function Select({ className = '', ...p }: ComponentProps<'select'>) {
  return <select {...p} className={`${fieldBox} ${fill(className)} h-8 ${className}`} />;
}

/** Titled bordered section of a page. */
export function Container({ title, actions, children }: { title: ReactNode; actions?: ReactNode; children: ReactNode }) {
  return (
    <section className="rounded border border-line bg-white">
      <div className="flex min-h-[45px] items-center justify-between gap-3 border-b border-line px-4 py-1.5">
        <h2 className="text-base font-bold">{title}</h2>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
      <div className="px-4 py-3">{children}</div>
    </section>
  );
}

/** Key-value grid used on overview pages. */
export function KV({ rows }: { rows: { label: ReactNode; value: ReactNode }[] }) {
  return (
    <dl className="grid grid-cols-[160px_1fr] gap-x-4 gap-y-2.5">
      {rows.map((r, i) => (
        <div key={i} className="contents">
          <dt className="text-text-3">{r.label}</dt>
          <dd className="min-w-0 break-words">{r.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Empty({ title, children }: { title: ReactNode; children?: ReactNode }) {
  return (
    <div className="px-5 py-10 text-center text-text-2">
      <b className="mb-1 block text-text">{title}</b>
      {children}
    </div>
  );
}
