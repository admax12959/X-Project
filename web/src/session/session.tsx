// Session state for the SPA. The cookie is HttpOnly, so the SPA learns whether it is
// logged in by asking GET /api/session. Any 401 from the API drops back to "anonymous",
// and RequireSession redirects to /login?next=<where you were>.
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';
import { api, ApiError, onSetupRequired, onUnauthenticated } from '../api/client.ts';
import type { LoginInput, User } from '../api/types.ts';
import { setLocale, useT } from '../i18n/index.ts';
import { ForcedChangePasswordPage } from '../pages/ChangePassword.tsx';
import { ForcedSecondFactorPage } from '../pages/ForcedSecondFactorPage.tsx';

/** The language saved on the user wins over this browser's choice, so it follows them across devices. */
function applySavedLanguage(user: User) {
  if (user.language === 'zh' || user.language === 'en') setLocale(user.language);
}

/** mustChangePassword: opened with an administrator-set password; RequireSession shows only the change-password page. secondFactorSetupRequired: a second factor is required and missing; it then shows only the setup page. */
type State = { status: 'loading' } | { status: 'anonymous' } | { status: 'error' } | { status: 'authenticated'; user: User; mustChangePassword: boolean; secondFactorSetupRequired: boolean };

type SessionApi = {
  state: State;
  /** 'secondFactor': the password was right but a code is still needed (finish with submitSecondFactor). */
  login: (input: LoginInput) => Promise<'done' | 'secondFactor'>;
  submitSecondFactor: (code: string) => Promise<void>;
  logout: () => Promise<void>;
  reload: () => void;
  /** Replace the cached session user after the user's own settings changed. */
  setUser: (user: User) => void;
};

const Ctx = createContext<SessionApi | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: 'loading' });

  const reload = useCallback(() => {
    setState({ status: 'loading' });
    api.session.get().then(
      (s) => {
        applySavedLanguage(s.user);
        setState({ status: 'authenticated', user: s.user, mustChangePassword: s.mustChangePassword, secondFactorSetupRequired: s.secondFactorSetupRequired });
      },
      (e: unknown) => setState(e instanceof ApiError && e.status === 401 ? { status: 'anonymous' } : { status: 'error' }),
    );
  }, []);

  useEffect(() => {
    onUnauthenticated(() => setState({ status: 'anonymous' }));
    onSetupRequired(reload);
    reload();
  }, [reload]);

  const login = useCallback(async (input: LoginInput) => {
    const s = await api.session.login(input);
    if ('secondFactorRequired' in s) return 'secondFactor' as const;
    applySavedLanguage(s.user);
    setState({ status: 'authenticated', user: s.user, mustChangePassword: s.mustChangePassword, secondFactorSetupRequired: s.secondFactorSetupRequired });
    return 'done' as const;
  }, []);

  const submitSecondFactor = useCallback(async (code: string) => {
    const s = await api.session.submitSecondFactor(code);
    applySavedLanguage(s.user);
    setState({ status: 'authenticated', user: s.user, mustChangePassword: s.mustChangePassword, secondFactorSetupRequired: s.secondFactorSetupRequired });
  }, []);

  const setUser = useCallback(
    (user: User) => setState((prev) => ({ status: 'authenticated', user, mustChangePassword: prev.status === 'authenticated' && prev.mustChangePassword, secondFactorSetupRequired: prev.status === 'authenticated' && prev.secondFactorSetupRequired })),
    [],
  );

  const logout = useCallback(async () => {
    await api.session.logout().catch(() => undefined);
    setState({ status: 'anonymous' });
  }, []);

  return <Ctx.Provider value={{ state, login, submitSecondFactor, logout, reload, setUser }}>{children}</Ctx.Provider>;
}

export function useSession(): SessionApi {
  const v = useContext(Ctx);
  if (!v) throw new Error('useSession outside SessionProvider');
  return v;
}

/** The logged-in user; only call below RequireSession. */
export function useCurrentUser(): User {
  const { state } = useSession();
  if (state.status !== 'authenticated') throw new Error('useCurrentUser without a session');
  return state.user;
}

function FullPage({ children }: { children: ReactNode }) {
  return <div className="flex min-h-screen items-center justify-center text-text-2">{children}</div>;
}

/** Guards every page except /login. */
export function RequireSession({ children }: { children: ReactNode }) {
  const { state, reload } = useSession();
  const { t } = useT();
  const location = useLocation();
  switch (state.status) {
    case 'loading':
      return <FullPage>{t('common.loading')}</FullPage>;
    case 'error':
      return (
        <FullPage>
          <span>
            {t('error.network')}{' '}
            <button type="button" className="text-accent hover:underline" onClick={reload}>
              {t('common.retry')}
            </button>
          </span>
        </FullPage>
      );
    case 'anonymous': {
      const next = location.pathname + location.search;
      return <Navigate to={next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`} replace />;
    }
    case 'authenticated':
      if (state.mustChangePassword) return <ForcedChangePasswordPage onDone={reload} />;
      return state.secondFactorSetupRequired ? <ForcedSecondFactorPage onDone={reload} /> : <>{children}</>;
  }
}
