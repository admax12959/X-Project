// A maintenance line's status as icon + text, never colour alone. Frozen is blue, not a
// warning colour: freezing is a normal state. The mainline has no status: render nothing.
import type { LineStatus } from '../api/types.ts';
import { useT, type MessageKey } from '../i18n/index.ts';

const STATUS: Record<LineStatus, { label: MessageKey; color: string }> = {
  active: { label: 'lineage.status.active', color: 'text-ok' },
  frozen: { label: 'lineage.status.frozen', color: 'text-[#0972d3]' },
  retired: { label: 'lineage.status.retired', color: 'text-text-3' },
};

function StatusIcon({ status }: { status: LineStatus }) {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden className="shrink-0">
      <circle cx="8" cy="8" r="6.25" />
      {status === 'active' && <path d="M5.2 8.2l2 2 3.6-4" />}
      {status === 'frozen' && <path d="M6.5 5.5v5M9.5 5.5v5" />}
      {status === 'retired' && <path d="M5.2 8h5.6" />}
    </svg>
  );
}

export function LineStatusLabel({ status }: { status: LineStatus | undefined }) {
  const { t } = useT();
  if (!status) return null;
  const s = STATUS[status];
  return (
    <span className={`inline-flex items-center gap-1.5 whitespace-nowrap ${s.color}`}>
      <StatusIcon status={status} />
      {t(s.label)}
    </span>
  );
}
