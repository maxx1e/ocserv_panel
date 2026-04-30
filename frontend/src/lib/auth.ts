import type { AuthSummary, Principal } from './api';
import { getMe, getSetupStatus, principalFromAuth } from './api';

export type AuthState = {
  setupRequired: boolean | null;
  principal: Principal | null;
  loading: boolean;
  error: string;
};

export function createAuthState(): AuthState {
  return {
    setupRequired: null,
    principal: null,
    loading: true,
    error: ''
  };
}

export function applyAuthSummary(state: AuthState, auth: AuthSummary | null): void {
  state.principal = principalFromAuth(auth);
}

export async function refreshAuthState(state: AuthState): Promise<void> {
  state.loading = true;
  state.error = '';

  try {
    const setup = await getSetupStatus();
    state.setupRequired = setup.required;

    if (setup.required) {
      state.principal = null;
      return;
    }

    try {
      applyAuthSummary(state, await getMe());
    } catch (error) {
      state.principal = null;
      if (error instanceof Error && !error.message.includes('Sign in is required')) {
        state.error = error.message;
      }
    }
  } catch (error) {
    state.error = error instanceof Error ? error.message : 'Unable to reach the backend API.';
  } finally {
    state.loading = false;
  }
}
