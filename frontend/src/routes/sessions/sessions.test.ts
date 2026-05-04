import { render, screen, waitFor } from '@testing-library/svelte';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import SessionsPage from './+page.svelte';
import { makeSession, makeViewerPrincipal, makeOwnerPrincipal, makeAuthSummary } from '$lib/test-utils';

// vi.mock is hoisted: intercepts $lib/api before the component imports it.
// importOriginal keeps hasOwnerRole, principalFromAuth, ApiError (class) intact.
vi.mock('$lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('$lib/api')>();
  return {
    ...actual,
    getSetupStatus: vi.fn().mockResolvedValue({ required: false }),
    getMe: vi.fn(),
    getOcservSessions: vi.fn().mockResolvedValue([]),
    getOcservSessionDetail: vi.fn(),
    disconnectOcservSession: vi.fn().mockResolvedValue(undefined),
    terminateOcservSession: vi.fn().mockResolvedValue(undefined)
  };
});

import { getMe, getOcservSessions } from '$lib/api';
const mockGetMe = vi.mocked(getMe);
const mockGetSessions = vi.mocked(getOcservSessions);

beforeEach(() => {
  vi.clearAllMocks();
});

describe('RBAC — viewer role', () => {
  it('hides Disconnect and Terminate buttons', async () => {
    mockGetMe.mockResolvedValue(makeAuthSummary(makeViewerPrincipal()));
    mockGetSessions.mockResolvedValue([makeSession()]);

    render(SessionsPage);

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /disconnect/i })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /terminate/i })).not.toBeInTheDocument();
    });
  });
});

describe('Terminate modal', () => {
  it('shows cookie-invalidation warning text', async () => {
    mockGetMe.mockResolvedValue(makeAuthSummary(makeOwnerPrincipal()));
    mockGetSessions.mockResolvedValue([makeSession({ username: 'alice' })]);

    render(SessionsPage);

    const terminateBtn = await screen.findByRole('button', { name: /^terminate$/i });
    await terminateBtn.click();

    await waitFor(() => {
      expect(screen.getByText(/invalidate the session cookie/i)).toBeInTheDocument();
    });
  });
});

describe('Disconnect modal', () => {
  it('shows reconnect-possible warning text', async () => {
    mockGetMe.mockResolvedValue(makeAuthSummary(makeOwnerPrincipal()));
    mockGetSessions.mockResolvedValue([makeSession({ username: 'alice' })]);

    render(SessionsPage);

    const disconnectBtn = await screen.findByRole('button', { name: /^disconnect$/i });
    await disconnectBtn.click();

    await waitFor(() => {
      expect(screen.getByText(/may reconnect if their session cookie is still valid/i)).toBeInTheDocument();
    });
  });
});
