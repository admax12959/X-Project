import { useId, useState, type ComponentProps, type MouseEvent, type ReactNode } from 'react';
import { Dialog } from '@base-ui/react/dialog';
import { Menu } from '@base-ui/react/menu';
import { Check, CircleCheck, CircleMinus, CirclePause, Copy, Ellipsis, Search } from 'lucide-react';
import { useStore, userName, type CodePlatform, type LineStatus, type User } from '../../data';
import { useT } from '../../i18n';
import { navigate } from '../../router';

export const cx = (...a: (string | false | null | undefined)[]) => a.filter(Boolean).join(' ');

// ---------- text atoms ----------

export const Mono = ({ children, className, title }: { children: ReactNode; className?: string; title?: string }) => (
  <span className={cx('cg-mono', className)} title={title}>
    {children}
  </span>
);

const STATUS_ICON = { Active: CircleCheck, Frozen: CirclePause, Retired: CircleMinus, Enabled: CircleCheck, Disabled: CircleMinus } as const;

/** Status is always icon + text, never a filled pill. */
export function Status({ status, size = 15 }: { status: LineStatus | User['status']; size?: number }) {
  const { t } = useT();
  const Icon = STATUS_ICON[status];
  const label = status === 'Enabled' || status === 'Disabled' ? t(`userStatus.${status}`) : t(`status.${status}`);
  return (
    <span className="cg-status" data-s={status}>
      <Icon size={size} aria-hidden />
      {label}
    </span>
  );
}

export const hostOf = (url: string) => url.replace(/^https?:\/\//, '');
export const platformLabel = (p: CodePlatform | undefined) => (p ? `${p.type} · ${hostOf(p.url)}` : '');

// ---------- links and buttons ----------

export function A({ to, children, className, title, ...rest }: { to: string; children: ReactNode; className?: string; title?: string } & Omit<ComponentProps<'a'>, 'href'>) {
  const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    navigate(to);
  };
  return (
    <a href={to} onClick={onClick} className={className} title={title} {...rest}>
      {children}
    </a>
  );
}

type BtnProps = ComponentProps<'button'> & { kind?: 'default' | 'primary' | 'danger'; sm?: boolean };
export function Btn({ kind = 'default', sm, className, type = 'button', ...p }: BtnProps) {
  return <button type={type} className={cx('cg-btn', kind === 'primary' && 'cg-btn-primary', kind === 'danger' && 'cg-btn-danger', sm && 'cg-btn-sm', className)} {...p} />;
}

export const LinkBtn = ({ className, type = 'button', ...p }: ComponentProps<'button'>) => <button type={type} className={cx('cg-linkbtn', className)} {...p} />;

/** Clipboard can reject in insecure contexts and headless browsers; copying is best-effort. */
export function CopyButton({ text, label }: { text: string; label?: string }) {
  const { t } = useT();
  const [done, setDone] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* best effort */
    }
    setDone(true);
    setTimeout(() => setDone(false), 1500);
  };
  const name = done ? t('common.copied') : (label ?? t('common.copy'));
  return (
    <button type="button" className="cg-icon-btn-sm" onClick={copy} aria-label={name} title={name} data-cg="copy">
      {done ? <Check size={14} /> : <Copy size={14} />}
    </button>
  );
}

// ---------- form fields ----------

export function Field({ label, error, hint, optional, className, children }: { label: ReactNode; error?: string; hint?: ReactNode; optional?: boolean; className?: string; children: ReactNode }) {
  const { t } = useT();
  return (
    <div className={cx('cg-field', className)}>
      <label>
        <span className="cg-label">
          {label}
          {optional && <span className="cg-label-opt">{t('common.optional')}</span>}
        </span>
        {children}
      </label>
      {error ? (
        <span className="cg-err" role="alert">
          {error}
        </span>
      ) : (
        hint && <span className="cg-hint">{hint}</span>
      )}
    </div>
  );
}
export const Input = ({ className, invalid, ...p }: ComponentProps<'input'> & { invalid?: boolean }) => <input className={cx('cg-input', className)} aria-invalid={invalid || undefined} {...p} />;
export const Select = ({ className, invalid, ...p }: ComponentProps<'select'> & { invalid?: boolean }) => <select className={cx('cg-select', className)} aria-invalid={invalid || undefined} {...p} />;
export const Textarea = ({ className, invalid, ...p }: ComponentProps<'textarea'> & { invalid?: boolean }) => <textarea className={cx('cg-textarea', className)} aria-invalid={invalid || undefined} {...p} />;

export function SearchBox({ value, onChange, placeholder, dataCg = 'search' }: { value: string; onChange: (v: string) => void; placeholder?: string; dataCg?: string }) {
  const { t } = useT();
  return (
    <span className="cg-search">
      <Search size={14} aria-hidden />
      <input type="search" value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder ?? t('common.search')} aria-label={placeholder ?? t('common.search')} data-cg={dataCg} />
    </span>
  );
}

// ---------- panels ----------

export function Panel({ title, actions, children, bodyClass = 'cg-panel-b', className, id, ...rest }: { title?: ReactNode; actions?: ReactNode; children: ReactNode; bodyClass?: string | null; className?: string; id?: string } & Record<`data-${string}`, string>) {
  return (
    <section className={cx('cg-panel', className)} id={id} {...rest}>
      {title !== undefined && (
        <div className="cg-panel-h">
          <h2>{title}</h2>
          {actions && <div className="cg-actions">{actions}</div>}
        </div>
      )}
      {bodyClass ? <div className={bodyClass}>{children}</div> : children}
    </section>
  );
}

export const Empty = ({ title, children }: { title: ReactNode; children?: ReactNode }) => (
  <div className="cg-empty">
    <strong>{title}</strong>
    {children}
  </div>
);

// ---------- dialog ----------

export function Modal({ title, description, onClose, footer, children, width = 560, dataCg }: { title: ReactNode; description?: ReactNode; onClose: () => void; footer?: ReactNode; children?: ReactNode; width?: number; dataCg?: string }) {
  return (
    <Dialog.Root open onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="cg-backdrop" />
        <Dialog.Popup className="cg-dlg" style={{ ['--cg-w' as string]: `${width}px` }} data-cg={dataCg}>
          <div className="cg-dlg-h">
            <Dialog.Title render={<h2 />}>{title}</Dialog.Title>
            {description && <Dialog.Description render={<p />}>{description}</Dialog.Description>}
          </div>
          {children && <div className="cg-dlg-b">{children}</div>}
          {footer && <div className="cg-dlg-f">{footer}</div>}
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** Confirmation with a primary (or danger) action. */
export function ConfirmDialog({ title, description, confirmLabel, danger, onConfirm, onClose, children }: { title: ReactNode; description?: ReactNode; confirmLabel: string; danger?: boolean; onConfirm: () => void; onClose: () => void; children?: ReactNode }) {
  const { t } = useT();
  return (
    <Modal
      title={title}
      description={description}
      onClose={onClose}
      width={480}
      dataCg="confirm"
      footer={
        <>
          <Btn onClick={onClose}>{t('common.cancel')}</Btn>
          <Btn kind={danger ? 'danger' : 'primary'} onClick={onConfirm} data-cg="confirm-ok" autoFocus>
            {confirmLabel}
          </Btn>
        </>
      }
    >
      {children}
    </Modal>
  );
}

// ---------- overflow menu ----------

export type MenuEntry = { label: string; onClick: () => void; danger?: boolean; dataCg?: string };
export function OverflowMenu({ items, label, dataCg = 'overflow' }: { items: MenuEntry[]; label: string; dataCg?: string }) {
  return (
    <Menu.Root>
      <Menu.Trigger className="cg-btn cg-icon-btn" aria-label={label} title={label} data-cg={dataCg}>
        <Ellipsis size={16} />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Positioner align="end" sideOffset={4} style={{ zIndex: 300 }}>
          <Menu.Popup className="cg-menu">
            {items.map((it) => (
              <Menu.Item key={it.label} className="cg-menu-item" data-danger={it.danger ? '' : undefined} data-cg={it.dataCg} onClick={it.onClick}>
                {it.label}
              </Menu.Item>
            ))}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

// ---------- people picker (Owners / PEs from Enabled users) ----------

export function PeoplePicker({ label, value, onChange, error, dataCg }: { label: string; value: string[]; onChange: (v: string[]) => void; error?: string; dataCg: string }) {
  const { t, tx } = useT();
  const s = useStore();
  const id = useId();
  const options = s.users.filter((u) => u.status === 'Enabled' && !value.includes(u.username));
  return (
    <div className="cg-field" data-cg={dataCg}>
      <span className="cg-label" id={id}>
        {label}
      </span>
      <div className="cg-people" role="group" aria-labelledby={id}>
        {value.length === 0 && <div className="cg-people-row cg-grey">{t('common.none')}</div>}
        {value.map((u) => (
          <div key={u} className="cg-people-row">
            <span>
              {userName(u)} <span className="cg-grey cg-mono">{u}</span>
            </span>
            <LinkBtn onClick={() => onChange(value.filter((x) => x !== u))} aria-label={`${t('catalog.remove')} ${userName(u)}`} data-cg="people-remove">
              {t('catalog.remove')}
            </LinkBtn>
          </div>
        ))}
        <div style={{ padding: 6 }}>
          <Select aria-label={`${t('catalog.add')}: ${label}`} value="" onChange={(e) => e.target.value && onChange([...value, e.target.value])} disabled={options.length === 0} data-cg="people-add">
            <option value="">{options.length ? t('catalog.addUser') : t('catalog.noUsersToAdd')}</option>
            {options.map((u) => (
              <option key={u.username} value={u.username}>
                {tx(u.displayName)} ({u.username})
              </option>
            ))}
          </Select>
        </div>
      </div>
      {error ? (
        <span className="cg-err" role="alert">
          {error}
        </span>
      ) : (
        <span className="cg-hint">{t('catalog.enabledOnly')}</span>
      )}
    </div>
  );
}
