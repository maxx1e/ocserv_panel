<script lang="ts">
  import { AlertTriangle, CheckCircle2, XCircle, Play } from 'lucide-svelte';
  import type { OcservReadiness } from '$lib/api';

  type Props = {
    readiness: OcservReadiness | null;
    loading: boolean;
    isOwner: boolean;
    onStartService?: () => void;
  };

  let { readiness, loading, isOwner, onStartService }: Props = $props();

  type CheckItem = {
    label: string;
    ok: boolean;
    detail: string;
  };

  const checks = $derived<CheckItem[]>(
    readiness
      ? [
          {
            label: 'ocserv binary',
            ok: readiness.binaryPresent,
            detail: readiness.binaryPresent
              ? `Found (${readiness.binaryVersion || 'version unknown'})`
              : 'Not found — verify ocserv is installed and its path is configured'
          },
          {
            label: 'occtl utility',
            ok: readiness.occtlPresent,
            detail: readiness.occtlPresent ? 'Found' : 'Not found — required for session management'
          },
          {
            label: 'ocpasswd utility',
            ok: readiness.ocpasswdPresent,
            detail: readiness.ocpasswdPresent ? 'Found' : 'Not found — required for user management'
          },
          {
            label: 'Configuration file',
            ok: readiness.configPresent && readiness.configValid,
            detail: !readiness.configPresent
              ? 'Missing — check the configured path'
              : readiness.configValid
                ? 'Present and valid'
                : 'Present but invalid — review configuration for errors'
          },
          {
            label: 'Service running',
            ok: readiness.serviceRunning,
            detail: readiness.serviceRunning ? 'Yes' : 'No — the ocserv daemon is not running'
          },
          {
            label: 'Control socket',
            ok: readiness.socketPresent,
            detail: readiness.socketPresent ? 'Found' : 'Not found — the management socket is unavailable'
          }
        ]
      : []
  );
</script>

{#if loading}
  <div class="bg-white border-l-4 border-amber-400 rounded-lg shadow-sm p-6">
    <div class="flex items-center gap-3">
      <div class="w-5 h-5 border-2 border-slate-200 border-t-amber-500 rounded-full animate-spin"></div>
      <span class="text-sm text-slate-600">Checking ocserv readiness...</span>
    </div>
  </div>

{:else if readiness === null}
  <div class="bg-white border-l-4 border-red-400 rounded-lg shadow-sm p-6">
    <div class="flex items-start gap-3">
      <XCircle class="w-5 h-5 text-red-500 mt-0.5 shrink-0" />
      <div>
        <h3 class="text-base font-semibold text-slate-800">Unable to retrieve diagnostics</h3>
        <p class="text-sm text-slate-600 mt-1">
          The readiness check could not be completed. The backend may be unreachable or misconfigured.
        </p>
      </div>
    </div>
  </div>

{:else}
  <div class="bg-white border-l-4 border-amber-400 rounded-lg shadow-sm p-6 space-y-5">
    <!-- Header -->
    <div class="flex items-start gap-3">
      <AlertTriangle class="w-5 h-5 text-amber-500 mt-0.5 shrink-0" />
      <div>
        <h3 class="text-base font-semibold text-slate-800">ocserv Service Diagnostics</h3>
        <p class="text-sm text-slate-600 mt-0.5">
          The VPN server is not currently available. Here's what we found:
        </p>
      </div>
    </div>

    <!-- Checklist -->
    <ul class="space-y-3 pl-1" role="list" aria-label="Readiness checks">
      {#each checks as item}
        <li class="flex items-start gap-3">
          {#if item.ok}
            <CheckCircle2 class="w-4.5 h-4.5 text-green-500 mt-0.5 shrink-0" />
          {:else}
            <XCircle class="w-4.5 h-4.5 text-red-500 mt-0.5 shrink-0" />
          {/if}
          <div class="min-w-0">
            <span class="text-sm font-medium text-slate-800">{item.label}</span>
            <span class="text-sm text-slate-500 ml-1.5">— {item.detail}</span>
          </div>
        </li>
      {/each}
    </ul>

    <!-- Start Service button (owner only, service not running) -->
    {#if isOwner && !readiness.serviceRunning && onStartService}
      <div class="pt-2 border-t border-slate-100">
        <button
          type="button"
          onclick={onStartService}
          class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md bg-slate-800 text-white hover:bg-slate-700 transition-colors"
        >
          <Play class="w-4 h-4" />
          Start ocserv
        </button>
      </div>
    {/if}

    <!-- Footer note -->
    <p class="text-xs text-slate-400 pt-1">
      Check the Audit tab for detailed backend logs.
    </p>
  </div>
{/if}
