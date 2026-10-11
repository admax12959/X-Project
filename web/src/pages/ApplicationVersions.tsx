// Versions tab: every version on every line of the application, latest release date
// first, filterable by line and by tag/commit. Owners get the Register version button.
// Filters run in the browser: the list is one application's versions.
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LineDetail, RefKind, Version } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty, Select } from '../ui/controls.tsx';
import { useApplication } from './ApplicationPage.tsx';
import { ForkDialog } from './ForkDialog.tsx';
import { RegisterVersionDialog } from './RegisterVersionDialog.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; versions: Version[]; lines: LineDetail[] };

export function ApplicationVersions() {
  const { t } = useT();
  const app = useApplication();
  const user = useCurrentUser();
  const navigate = useNavigate();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [lineId, setLineId] = useState('');
  const [refKind, setRefKind] = useState<RefKind | ''>('');
  const [registering, setRegistering] = useState(false);
  const [forkFrom, setForkFrom] = useState<string | null>(null);

  const reload = useCallback(() => {
    let live = true;
    Promise.all([api.applications.versions(app.key), api.applications.lines(app.key)]).then(
      ([v, l]) => live && setLoad({ status: 'ok', versions: v.items, lines: l.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [app.key]);
  useEffect(() => reload(), [reload]);

  // Whether to offer the button is decided here only for display; the server decides.
  const isOwner = app.owners.some((o) => o.id === user.id);
  const versions = load.status === 'ok' ? load.versions : [];
  const lines = load.status === 'ok' ? load.lines : [];
  const shown = versions.filter((v) => (!lineId || v.lineId === lineId) && (!refKind || v.ref.kind === refKind));
  const target = lines.find((l) => l.isMain);
  const filtered = lineId !== '' || refKind !== '';
  const th = 'h-10 whitespace-nowrap border-b border-line bg-thead px-3 text-left text-[13px] font-semibold';
  const td = 'border-b border-sep px-3 py-2 align-top';
  const lineName = (l: Pick<LineDetail, 'isMain' | 'name'>) => (l.isMain ? t('term.mainline') : l.name);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Select aria-label={t('versions.filter.line')} className="w-56" value={lineId} onChange={(e) => setLineId(e.target.value)}>
          <option value="">{t('versions.filter.allLines')}</option>
          {lines.map((l) => (
            <option key={l.id} value={l.id}>
              {lineName(l)} · {l.branch}
            </option>
          ))}
        </Select>
        <Select aria-label={t('versions.filter.ref')} className="w-44" value={refKind} onChange={(e) => setRefKind(e.target.value as RefKind | '')}>
          <option value="">{t('versions.filter.allRefs')}</option>
          <option value="tag">{t('versions.ref.tag')}</option>
          <option value="commit">{t('versions.ref.commit')}</option>
        </Select>
        {filtered && (
          <Button
            small
            onClick={() => {
              setLineId('');
              setRefKind('');
            }}
          >
            {t('versions.filter.clear')}
          </Button>
        )}
        <span className="flex-1" />
        {load.status === 'ok' && versions.length > 0 && (
          <span className="text-[13px] text-text-3">{t('versions.count', { shown: shown.length, total: versions.length })}</span>
        )}
        {isOwner && target && (
          <Button kind="primary" onClick={() => setRegistering(true)}>
            {t('versions.register')}
          </Button>
        )}
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('versions.col.version')}</th>
              <th className={th}>{t('versions.col.line')}</th>
              <th className={th}>{t('versions.col.ref')}</th>
              <th className={th}>{t('versions.col.released')}</th>
              <th className={th}>{t('versions.col.note')}</th>
              {isOwner && <th className={th} />}
            </tr>
          </thead>
          <tbody>
            {shown.map((v) => (
              <tr key={v.id}>
                <td className={`${td} whitespace-nowrap font-mono text-[12.5px]`}>{v.number}</td>
                <td className={td}>
                  {lineName(v.line)} <span className="font-mono text-[12.5px] text-text-3">{v.line.branch}</span>
                </td>
                <td className={`${td} whitespace-nowrap`}>
                  <span className="text-text-3">{t(v.ref.kind === 'tag' ? 'versions.ref.tag' : 'versions.ref.commit')}</span>{' '}
                  <span className="font-mono text-[12.5px]">{v.ref.value}</span>
                </td>
                <td className={`${td} whitespace-nowrap`}>{v.releasedAt}</td>
                <td className={`${td} whitespace-pre-wrap`}>{v.note}</td>
                {isOwner && (
                  <td className={`${td} whitespace-nowrap`}>
                    <Button small onClick={() => setForkFrom(v.id)}>
                      {t('lineage.panel.forkFromVersion')}
                    </Button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('versions.loadFailed')} />}
        {load.status === 'ok' && versions.length === 0 && (
          <Empty title={t('versions.empty.title')}>{isOwner ? t('versions.empty.bodyOwner') : t('versions.empty.body')}</Empty>
        )}
        {load.status === 'ok' && versions.length > 0 && shown.length === 0 && <Empty title={t('versions.noMatch')} />}
      </div>

      {forkFrom && (
        <ForkDialog
          appKey={app.key}
          versions={versions}
          lines={lines}
          sourceId={forkFrom}
          onClose={() => setForkFrom(null)}
          // Show the new line in the lineage, selected.
          onForked={(line) => navigate(`/apps/${encodeURIComponent(app.key)}/lineage?line=${line.id}`)}
        />
      )}

      {registering && target && (
        <RegisterVersionDialog
          appKey={app.key}
          line={target}
          // The list is sorted latest release date first, so the first version of the line is its previous one.
          previous={versions.find((v) => v.lineId === target.id)}
          onClose={() => setRegistering(false)}
          onRegistered={() => {
            setRegistering(false);
            reload();
          }}
        />
      )}
    </div>
  );
}
