<script lang="ts">
  import { AlertTriangle, RefreshCw } from 'lucide-svelte';
  import type { AuditLogSource } from '$lib/api';

  type Props = {
    source: AuditLogSource;
    onRetry: () => void;
  };

  let { source, onRetry }: Props = $props();

  const reasonMessages: Record<string, string> = {
    service_not_running:
      'The service is not running or the log source is not accessible.',
    file_not_found: 'The log file does not exist at the configured path.',
    permission_denied:
      'The log file exists but cannot be read. Check file permissions.'
  };

  const message = $derived(
    reasonMessages[source.reason] ??
      'The log source is not available. Check service status and configuration.'
  );

  const settingsHint = $derived(
    source.name === 'panel'
      ? 'Settings → Installation → Panel Log Path'
      : 'Settings → Installation → ocserv Log Path'
  );
</script>

<div
  class="rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/30 p-5"
>
  <div class="flex items-start gap-3">
    <AlertTriangle class="w-5 h-5 text-amber-500 dark:text-amber-400 mt-0.5 shrink-0" />
    <div class="flex-1 min-w-0">
      <h3 class="text-sm font-semibold text-amber-800 dark:text-amber-200">
        {source.label} Unavailable
      </h3>
      <p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
        {message}
      </p>
      <div class="mt-3 space-y-1">
        <p class="text-xs text-amber-600 dark:text-amber-400">
          <span class="font-medium">Expected path:</span>
          <code class="ml-1 px-1.5 py-0.5 bg-amber-100 dark:bg-amber-900/50 rounded text-amber-800 dark:text-amber-200">
            {source.path}
          </code>
        </p>
        <p class="text-xs text-amber-600 dark:text-amber-400">
          <span class="font-medium">Configured via:</span>
          <span class="ml-1">{settingsHint}</span>
        </p>
      </div>
      <button
        type="button"
        onclick={onRetry}
        class="mt-4 inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-amber-700 dark:text-amber-200 bg-amber-100 dark:bg-amber-900/50 hover:bg-amber-200 dark:hover:bg-amber-900 rounded-md transition-colors"
      >
        <RefreshCw class="w-3.5 h-3.5" />
        Retry
      </button>
    </div>
  </div>
</div>
