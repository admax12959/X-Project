// Session state for the SPA. The cookie is HttpOnly, so the SPA learns whether it is
// logged in by asking GET /api/session. Any 401 from the API drops back to "anonymous",
// and RequireSession redirects to /login?next=<where you were>.
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router';
import { api, ApiError, onUnauthenticated } from '../api/client.ts';
import type { LoginInput, User } from '../api/types.ts';
import { useT } from '../i18n/index.ts';

type State = { status: 'loading' } | { status: 'anonymous' } | { status: 'error' } | { status: 'authenticated'; user: User };

type SessionApi = {
  state: State;
  login: (input: LoginInput) => Promise<void>;
  logout: () => Promise<void>;
  reload: () => void;
};

const Ctx = createContext<SessionApi | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: 'loading' });

  const reload = useCallback(() => {
    setState({ status: 'loading' });
    api.session.get().then(
      (s) => setState({ status: 'authenticated', user: s.user }),
      (e: unknown) => setState(e instanceof ApiError && e.status === 401 ? { status: 'anonymous' } : { status: 'error' }),
    );
  }, []);

  useEffect(() => {
    onUnauthenticated(() => setState({ status: 'anonymous' }));
    reload();
  }, [reload]);

  const login = useCallback(async (input: LoginInput) => {
    const s = await api.session.login(input);
    setState({ status: 'authenticated', user: s.user });
  }, []);

  const logout = useCallback(async () => {
    await api.session.logout().catch(() => undefined);
    setState({ status: 'anonymous' });
  }, []);

  return <Ctx.Provider value={{ state, login, logout, reload }}>{children}</Ctx.Provider>;
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
      return <>{children}</>;
  }
}
