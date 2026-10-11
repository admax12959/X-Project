// Recent activity under the application list: the latest change events of all applications,
// newest first. Sentences come from the same renderer as the change-history tab.
import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { RecentActivity as Activity } from '../api/types.ts';
import { renderEvent } from '../history/render.ts';
import { useT } from '../i18n/index.ts';
import { Empty } from '../ui/controls.tsx';
import { appPath } from '../ui/links.ts';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: Activity[] };

export function RecentActivity() {
  const { t, locale } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });

  useEffect(() => {
    let live = true;
    api.activity.recent().then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, []);

  const items = load.status === 'ok' ? load.items : [];
  const when = new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' });
  const th = 'h-10 whitespace-nowrap border-b border-line bg-thead px-3 text-left text-[13px] font-semibold';
  const td = 'border-b border-sep px-3 py-2 align-top';

  return (
    <section aria-labelledby="recent-activity" className="space-y-2">
      <h2 id="recent-activity" className="text-lg font-bold">
        {t('apps.activity.title')}
      </h2>
      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('history.col.time')}</th>
              <th className={th}>{t('term.application')}</th>
              <th className={th}>{t('history.col.by')}</th>
              <th className={th}>{t('history.col.event')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((e) => (
              <tr key={e.id}>
                <td className={`${td} whitespace-nowrap`}>{when.format(new Date(e.at))}</td>
                <td className={`${td} whitespace-nowrap`}>
                  <Link to={appPath(e.application.key)} className="text-accent hover:underline">
                    {e.application.name}
                  </Link>
                </td>
                <td className={`${td} whitespace-nowrap`}>{e.actor.displayName}</td>
                <td className={td}>{renderEvent(e, locale)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('apps.activity.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && <Empty title={t('apps.activity.empty')} />}
      </div>
    </section>
  );
}
