// Right-hand detail panel of the Lineage tab: one line's facts and its versions, with
// Register version for that line and Fork from any of its versions (Owners). The planned
// end date is one plain row: no countdown, colour or reminder. Rendered beside the tree
// (not as a shell-level panel) so the tree can use the width that is left.
import type { ReactNode } from 'react';
import type { LineDetail, Version } from '../api/types.ts';
import { useT, type MessageKey } from '../i18n/index.ts';
import { Button } from '../ui/controls.tsx';
import { LineStatusLabel } from '../ui/LineStatusLabel.tsx';

const HINT: Record<NonNullable<LineDetail['status']>, MessageKey> = {
  active: 'lineage.status.active.hint',
  frozen: 'lineage.status.frozen.hint',
  retired: 'lineage.status.retired.hint',
};

export function LinePanel({
  line,
  versions,
  isOwner,
  onClose,
  onSelectLine,
  onFork,
  onRegister,
}: {
  line: LineDetail;
  /** This line's versions, latest release date first. */
  versions: Version[];
  isOwner: boolean;
  onClose: () => void;
  onSelectLine: (lineId: string) => void;
  /** Fork from this version. */
  onFork: (versionId: string) => void;
  onRegister: () => void;
}) {
  const { t, locale } = useT();
  const retired = line.status === 'retired';
  const when = new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' });
  const none = <span className="text-text-3">{t('lineage.field.none')}</span>;
  const sv = line.sourceVersion;
  const rows: { label: string; value: ReactNode }[] = [
    { label: t('lineage.field.branch'), value: <span className="font-mono text-[12.5px]">{line.branch}</span> },
  ];
  if (!line.isMain) {
    rows.push(
      {
        label: t('lineage.field.source'),
        value: sv ? (
          <>
            <span className="font-mono text-[12.5px]">{sv.number}</span>
            {', '}
            <button type="button" className="text-accent hover:underline" onClick={() => onSelectLine(sv.line.id)}>
              {sv.line.isMain ? t('term.mainline') : sv.line.name}
            </button>
          </>
        ) : (
          none
        ),
      },
      { label: t('lineage.field.reason'), value: line.reason ? <span className="whitespace-pre-wrap">{line.reason}</span> : none },
      { label: t('lineage.field.forkedAt'), value: line.forkedAt ? when.format(new Date(line.forkedAt)) : none },
      { label: t('lineage.field.plannedEnd'), value: line.plannedEnd || none },
      { label: t('lineage.field.note'), value: line.note ? <span className="whitespace-pre-wrap">{line.note}</span> : none },
    );
  }

  return (
    <aside aria-label={t('lineage.panel')} className="min-w-0 space-y-4 rounded border border-line bg-white p-4">
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <h2 className="break-words text-base font-bold">{line.isMain ? t('term.mainline') : line.name}</h2>
          {!line.isMain && <div className="text-[13px] text-text-3">{t('lineage.panel.kind')}</div>}
        </div>
        <Button small aria-label={t('lineage.panel.close')} onClick={onClose}>
          ×
        </Button>
      </div>

      {isOwner && (
        <div className="flex flex-wrap gap-2">
          <Button
            small
            disabled={versions.length === 0}
            title={versions.length === 0 ? t('lineage.panel.forkNoVersion') : undefined}
            onClick={() => versions[0] && onFork(versions[0].id)}
          >
            {t('lineage.panel.fork')}
          </Button>
          <Button small kind="primary" disabled={retired} title={retired ? t('lineage.panel.retiredHint') : undefined} onClick={onRegister}>
            {t('lineage.panel.register')}
          </Button>
        </div>
      )}

      {line.status && (
        <div>
          <div className="mb-1 text-[13px] font-semibold">{t('lineage.col.status')}</div>
          <LineStatusLabel status={line.status} />
          <p className="mt-0.5 text-[13px] text-text-3">{t(HINT[line.status])}</p>
        </div>
      )}

      <section>
        <h3 className="mb-1.5 text-[13px] font-semibold">{t('lineage.panel.details')}</h3>
        <dl className="grid grid-cols-[110px_minmax(0,1fr)] gap-x-3 gap-y-2 text-[13px]">
          {rows.map((r) => (
            <div key={r.label} className="contents">
              <dt className="text-text-3">{r.label}</dt>
              <dd className="min-w-0 break-words">{r.value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section>
        <h3 className="mb-1.5 text-[13px] font-semibold">{t('lineage.panel.versions', { count: versions.length })}</h3>
        {versions.length === 0 ? (
          <p className="text-[13px] text-text-3">{t(isOwner ? 'lineage.panel.noVersionsOwner' : 'lineage.panel.noVersions')}</p>
        ) : (
          <ul className="divide-y divide-sep rounded border border-sep">
            {versions.map((v) => (
              <li key={v.id} className="px-2.5 py-2 text-[13px]">
                <div className="flex items-baseline gap-2">
                  <span className="font-mono text-[12.5px] font-semibold">{v.number}</span>
                  <span className="text-text-3">{v.releasedAt}</span>
                </div>
                <div className="text-text-3">
                  {t(v.ref.kind === 'tag' ? 'versions.ref.tag' : 'versions.ref.commit')} <span className="font-mono text-[12.5px]">{v.ref.value}</span>
                </div>
                {isOwner && (
                  <button type="button" className="mt-0.5 text-accent hover:underline" onClick={() => onFork(v.id)}>
                    {t('lineage.panel.forkFromVersion')}
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </aside>
  );
}
