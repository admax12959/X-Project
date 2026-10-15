// Change history tab: time / by / event. Sentences are rendered here from type + params
// (src/history/render.ts) in the current UI language; user input is shown verbatim.
import { useEffect, useState } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { ChangeEvent } from '../api/types.ts';
import { renderEvent } from '../history/render.ts';
import { useT } from '../i18n/index.ts';
import { Empty } from '../ui/controls.tsx';
import { useApplication } from './ApplicationPage.tsx';
import { th, tdTop as td } from '../ui/table.ts';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: ChangeEvent[] };

export function ApplicationHistory() {
  const { t, locale } = useT();
  const app = useApplication();
  const [load, setLoad] = useState<Load>({ status: 'loading' });

  useEffect(() => {
    let live = true;
    api.applications.history(app.key).then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [app.key]);

  const items = load.status === 'ok' ? load.items : [];
  const when = new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <div className="overflow-x-auto rounded border border-line bg-white">
      <table className="w-full border-collapse">
        <thead>
          <tr>
            <th className={th}>{t('history.col.time')}</th>
            <th className={th}>{t('history.col.by')}</th>
            <th className={th}>{t('history.col.event')}</th>
          </tr>
        </thead>
        <tbody>
          {items.map((e) => (
            <tr key={e.id}>
              <td className={`${td} whitespace-nowrap`}>{when.format(new Date(e.at))}</td>
              <td className={`${td} whitespace-nowrap`}>{e.actor.displayName}</td>
              <td className={td}>{renderEvent(e, locale)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('history.loadFailed')} />}
      {load.status === 'ok' && items.length === 0 && <Empty title={t('history.empty')} />}
    </div>
  );
}
