<script lang="ts">
  import { AlertTriangle } from 'lucide-svelte';

  type Props = {
    logLevel: number | null;
    logLevelLabel: string | null;
  };

  let { logLevel, logLevelLabel }: Props = $props();

  const isSensitive = $derived(logLevel !== null && logLevel >= 8);

  const levelDescriptions: Record<number, string> = {
    0: 'default/info',
    1: 'basic',
    2: 'info',
    3: 'debug',
    4: 'http',
    8: 'sensitive (may contain credentials)',
    9: 'TLS (may contain keys and handshake data)'
  };
</script>

{#if isSensitive}
  <div class="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/30 p-4">
    <AlertTriangle class="w-5 h-5 text-amber-600 dark:text-amber-400 shrink-0 mt-0.5" />
    <div>
      <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
        ocserv log-level is set to {logLevel} ({logLevelLabel ?? levelDescriptions[logLevel ?? 0] ?? 'unknown'})
      </p>
      <p class="text-xs text-amber-700 dark:text-amber-300 mt-1">
        Logs at this level may contain sensitive data including credentials, session tokens, and TLS handshake details. Exercise caution when sharing or exporting.
      </p>
    </div>
  </div>
{:else if logLevel !== null}
  <div class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
    <span>ocserv log-level: {logLevel} ({logLevelLabel ?? levelDescriptions[logLevel] ?? 'unknown'})</span>
  </div>
{/if}
