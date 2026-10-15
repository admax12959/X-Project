// Security tab content: turning on two-step verification with an authenticator app.
// Flow: type the password again -> scan the QR code (or type the key) -> enter one code to confirm.
import { useEffect, useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { SecondFactorStatus, TotpSetup } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input } from '../ui/controls.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; value: SecondFactorStatus };

function ErrorLine({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
      {message}
    </p>
  );
}

/** The QR code is shown as an image, so the SVG from the server is never part of the page's DOM. */
function qrSrc(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function SecondFactorPanel() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  // Set once the password was accepted; cleared when the setup is cancelled or confirmed.
  const [setup, setSetup] = useState<TotpSetup | null>(null);
  const [asking, setAsking] = useState(false);

  useEffect(() => {
    let live = true;
    api.me.secondFactor.get().then(
      (value) => live && setLoad({ status: 'ok', value }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, []);

  if (load.status === 'loading') return <p className="text-text-2">{t('common.loading')}</p>;
  if (load.status === 'error') return <ErrorLine message={errorsByField(load.error)[''] ?? t('settings.loadFailed')} />;

  const status = load.value;
  const finish = (value: SecondFactorStatus) => {
    setSetup(null);
    setAsking(false);
    setLoad({ status: 'ok', value });
  };

  if (status.enabled) {
    return (
      <div role="status" className="grid gap-1">
        <p className="font-semibold text-ok">{t('settings.security.totp.on')}</p>
        <p className="text-[13px] text-text-2">{t('settings.security.totp.onHint')}</p>
      </div>
    );
  }
  if (setup) return <ConfirmStep setup={setup} onCancel={() => setSetup(null)} onDone={finish} />;
  if (asking) return <PasswordStep onCancel={() => setAsking(false)} onStarted={setSetup} />;
  return (
    <div className="grid max-w-[480px] gap-3">
      <p className="font-semibold">{t('settings.security.totp.title')}</p>
      <p className="text-[13px] text-text-2">{t('settings.security.totp.hint')}</p>
      <div>
        <Button kind="primary" onClick={() => setAsking(true)}>
          {t('settings.security.totp.enable')}
        </Button>
      </div>
    </div>
  );
}

function PasswordStep({ onCancel, onStarted }: { onCancel: () => void; onStarted: (s: TotpSetup) => void }) {
  const { t } = useT();
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onStarted(await api.me.secondFactor.startTotp(password));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate className="grid max-w-[380px] gap-3.5">
      <p className="text-[13px] text-text-2">{t('settings.security.totp.passwordHint')}</p>
      <ErrorLine message={errors['']} />
      <Field label={t('settings.security.totp.password')} error={errors.password}>
        <Input name="password" type="password" autoComplete="current-password" autoFocus value={password} onChange={(e) => setPassword(e.target.value)} />
      </Field>
      <div className="flex gap-2">
        <Button kind="primary" type="submit" disabled={busy}>
          {t('settings.security.totp.next')}
        </Button>
        <Button type="button" onClick={onCancel}>
          {t('settings.security.totp.cancel')}
        </Button>
      </div>
    </form>
  );
}

function ConfirmStep({ setup, onCancel, onDone }: { setup: TotpSetup; onCancel: () => void; onDone: (s: SecondFactorStatus) => void }) {
  const { t } = useT();
  const [code, setCode] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onDone(await api.me.secondFactor.confirmTotp(code));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate className="grid max-w-[480px] gap-3.5">
      <p className="text-[13px] text-text-2">{t('settings.security.totp.scan')}</p>
      <img src={qrSrc(setup.qrSvg)} alt={t('settings.security.totp.qrAlt')} width={196} height={196} className="rounded border border-line" />
      <div>
        <p className="mb-1 text-[13px] font-semibold">{t('settings.security.totp.manualKey')}</p>
        <code className="block break-all rounded border border-line bg-subtle px-3 py-2 font-mono text-[13px]">{setup.secret.replace(/(.{4})/g, '$1 ').trim()}</code>
        <p className="mt-1 text-[12px] text-text-3">{t('settings.security.totp.manualKeyHint')}</p>
      </div>
      <ErrorLine message={errors['']} />
      <Field label={t('settings.security.totp.code')} error={errors.code}>
        <Input name="code" inputMode="numeric" autoComplete="one-time-code" maxLength={7} value={code} onChange={(e) => setCode(e.target.value)} />
      </Field>
      <div className="flex gap-2">
        <Button kind="primary" type="submit" disabled={busy}>
          {t('settings.security.totp.confirm')}
        </Button>
        <Button type="button" onClick={onCancel}>
          {t('settings.security.totp.cancel')}
        </Button>
      </div>
    </form>
  );
}
