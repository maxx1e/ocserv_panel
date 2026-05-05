<script lang="ts">
  import { onMount } from 'svelte';
  import { FileText, History, Network, Check, AlertTriangle, X, Copy, RotateCcw, Eye, FileDiff, RefreshCw } from 'lucide-svelte';
  import ConfigEditor from '$lib/components/ui/ConfigEditor.svelte';
  import ConfigIssueList from '$lib/components/ui/ConfigIssueList.svelte';
  import {
    getConfig, validateConfig, diffConfig, applyConfig,
    getConfigBackups, getConfigBackup, diffConfigBackup,
    rollbackConfig, getNetworkPrerequisites,
    getMe, principalFromAuth, hasOwnerRole,
    type ConfigIssue, type ConfigMetadata,
    type ConfigBackupInfo, type NetworkPrerequisites, type NetworkPrerequisiteCheck,
    type Principal, ApiError
  } from '$lib/api';

  // ─── Auth & RBAC ─────────────────────────────────────────────
  let principal = $state<Principal | null>(null);
  const isOwner = $derived(hasOwnerRole(principal));

  // ─── Editor ref ──────────────────────────────────────────────
  let editorRef = $state<ConfigEditor | undefined>(undefined);

  // ─── Tabs ────────────────────────────────────────────────────
  let activeTab = $state<'editor' | 'backups' | 'network'>('editor');
  let backupsLoaded = $state(false);
  let networkLoaded = $state(false);

  // ─── Editor ──────────────────────────────────────────────────
  let loading = $state(true);
  let error = $state('');
  let configContent = $state('');
  let editContent = $state('');
  let configMeta = $state<ConfigMetadata | null>(null);
  let issues = $state<ConfigIssue[]>([]);
  let validating = $state(false);
  let applying = $state(false);
  let diffText = $state('');
  let showDiffModal = $state(false);
  let showApplyConfirm = $state(false);
  let requiresRestart = $state(false);
  let restartReasons = $state<string[]>([]);
  let lastValidatedContent = $state('');
  let successBanner = $state('');
  let errorBanner = $state('');
  const hasChanges = $derived(editContent !== configContent);

  // ─── Backups ─────────────────────────────────────────────────
  let backups = $state<ConfigBackupInfo[]>([]);
  let loadingBackups = $state(false);
  let backupView = $state<{ n: number; content: string; mode: 'view' | 'diff' } | null>(null);
  let loadingBackupDetail = $state(false);
  let rolling = $state(false);
  let rollbackConfirmN = $state<number | null>(null);

  // ─── Network ─────────────────────────────────────────────────
  let network = $state<NetworkPrerequisites | null>(null);
  let loadingNetwork = $state(false);
  let copiedSuggestion = $state('');

  // ─── Banner timers ───────────────────────────────────────────
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  let errorTimer: ReturnType<typeof setTimeout> | null = null;

  // ─── Lifecycle ───────────────────────────────────────────────

  onMount(async () => {
    try {
      principal = principalFromAuth(await getMe());
    } catch { /* ignore */ }
    await loadConfig();
  });

  // ─── Data Loading ────────────────────────────────────────────

  async function loadConfig() {
    loading = true;
    error = '';
    try {
      const result = await getConfig();
      configContent = result.content;
      editContent = result.content;
      configMeta = result.metadata;
      issues = result.issues;
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 503) {
          error = 'ocserv is not available. Is the service running?';
        } else if (caught.status === 403) {
          error = 'Your role does not have permission to view configuration.';
        } else {
          error = caught.message;
        }
      } else {
        error = caught instanceof Error ? caught.message : 'Unable to load configuration.';
      }
    } finally {
      loading = false;
    }
  }

  function switchTab(tab: 'editor' | 'backups' | 'network') {
    activeTab = tab;
    if (tab === 'backups' && !backupsLoaded) {
      void loadBackups();
    }
    if (tab === 'network' && !networkLoaded) {
      void loadNetwork();
    }
  }

  // ─── Editor Actions ──────────────────────────────────────────

  async function handleValidate() {
    if (validating) return;
    validating = true;
    issues = [];
    try {
      const result = await validateConfig(editContent);
      issues = result.issues;
      requiresRestart = result.requiresRestart ?? false;
      restartReasons = result.restartReasons ?? [];
      lastValidatedContent = editContent;
      if (result.valid) {
        showSuccess('Configuration is valid.');
      } else {
        const errorCount = result.issues.filter(i => i.severity === 'error').length;
        const warnCount = result.issues.filter(i => i.severity === 'warning').length;
        showError(`Validation found ${errorCount} error(s) and ${warnCount} warning(s).`);
      }
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : 'Validation request failed.');
    } finally {
      validating = false;
    }
  }

  async function handlePreview() {
    if (!hasChanges) return;
    try {
      const result = await diffConfig(editContent);
      diffText = result.diff;
      showDiffModal = true;
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : 'Diff request failed.');
    }
  }

  async function requestApply() {
    // Auto-validate if content changed since last validation
    if (editContent !== lastValidatedContent) {
      await handleValidate();
      // If validation found errors, block apply
      if (issues.some(i => i.severity === 'error')) {
        showError('Fix validation errors before applying.');
        return;
      }
    } else if (issues.some(i => i.severity === 'error')) {
      // Content hasn't changed but there are known errors
      showError('Fix validation errors before applying.');
      return;
    }
    // Fetch diff for display in confirmation
    try {
      const result = await diffConfig(editContent);
      diffText = result.diff;
    } catch {
      diffText = '';
    }
    showApplyConfirm = true;
  }

  function handleAutoFix(issue: ConfigIssue) {
    if (!issue.line || !issue.fixedValue) return;
    const lines = editContent.split('\n');
    const lineIdx = issue.line - 1;
    if (lineIdx < 0 || lineIdx >= lines.length) return;
    lines[lineIdx] = issue.fixedValue;
    editContent = lines.join('\n');
    // Re-validate after fix
    void handleValidate();
  }

  async function handleApply() {
    if (applying) return;
    applying = true;
    showApplyConfirm = false;
    try {
      const result = await applyConfig(editContent);
      configContent = editContent;
      issues = result.issues;
      if (result.reloadSuccess) {
        showSuccess(`Configuration applied successfully. Backup #${result.backupNumber} created.`);
      } else {
        showSuccess(`Configuration saved (backup #${result.backupNumber}), but reload had issues: ${result.reloadError ?? 'unknown'}`);
      }
      // Refresh backups if already loaded
      if (backupsLoaded) {
        void loadBackups();
      }
    } catch (caught) {
      if (caught instanceof ApiError) {
        if (caught.status === 409) {
          showError('Configuration is locked by another operation. Try again shortly.');
        } else if (caught.status === 422) {
          showError('Configuration has validation errors. Fix them before applying.');
        } else {
          showError(caught.message);
        }
      } else {
        showError(caught instanceof Error ? caught.message : 'Apply request failed.');
      }
    } finally {
      applying = false;
    }
  }

  // ─── Backups ─────────────────────────────────────────────────

  async function loadBackups() {
    loadingBackups = true;
    try {
      backups = await getConfigBackups();
      backupsLoaded = true;
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : 'Unable to load backups.');
    } finally {
      loadingBackups = false;
    }
  }

  async function viewBackup(n: number, mode: 'view' | 'diff') {
    loadingBackupDetail = true;
    backupView = null;
    try {
      if (mode === 'view') {
        const result = await getConfigBackup(n);
        backupView = { n, content: result.content, mode: 'view' };
      } else {
        const result = await diffConfigBackup(n);
        backupView = { n, content: result.diff, mode: 'diff' };
      }
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : `Unable to load backup #${n}.`);
    } finally {
      loadingBackupDetail = false;
    }
  }

  function requestRollback(n: number) {
    rollbackConfirmN = n;
  }

  async function handleRollback() {
    if (rolling || rollbackConfirmN === null) return;
    rolling = true;
    const n = rollbackConfirmN;
    rollbackConfirmN = null;
    try {
      const result = await rollbackConfig(n);
      if (result.reloadSuccess) {
        showSuccess(`Rolled back to backup #${n} successfully.`);
      } else {
        showSuccess(`Rolled back to backup #${n}, but reload had issues: ${result.reloadError ?? 'unknown'}`);
      }
      // Reload config to reflect changes
      await loadConfig();
      if (backupsLoaded) {
        void loadBackups();
      }
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 409) {
        showError('Configuration is locked by another operation. Try again shortly.');
      } else {
        showError(caught instanceof Error ? caught.message : `Rollback to backup #${n} failed.`);
      }
    } finally {
      rolling = false;
    }
  }

  // ─── Network ─────────────────────────────────────────────────

  async function loadNetwork() {
    loadingNetwork = true;
    try {
      network = await getNetworkPrerequisites();
      networkLoaded = true;
    } catch (caught) {
      showError(caught instanceof Error ? caught.message : 'Unable to load network prerequisites.');
    } finally {
      loadingNetwork = false;
    }
  }

  function copySuggestion(text: string) {
    navigator.clipboard.writeText(text);
    copiedSuggestion = text;
    setTimeout(() => {
      copiedSuggestion = '';
    }, 2000);
  }

  // ─── Helpers ─────────────────────────────────────────────────

  function showSuccess(msg: string) {
    successBanner = msg;
    if (successTimer) clearTimeout(successTimer);
    successTimer = setTimeout(() => { successBanner = ''; }, 5000);
  }

  function showError(msg: string) {
    errorBanner = msg;
    if (errorTimer) clearTimeout(errorTimer);
    errorTimer = setTimeout(() => { errorBanner = ''; }, 8000);
  }

  function renderDiffLine(line: string): { cls: string; text: string } {
    if (line.startsWith('---') || line.startsWith('+++')) return { cls: 'text-slate-600 font-semibold', text: line };
    if (line.startsWith('-')) return { cls: 'bg-red-50 text-red-800', text: line };
    if (line.startsWith('+')) return { cls: 'bg-green-50 text-green-800', text: line };
    if (line.startsWith('@@')) return { cls: 'text-blue-500', text: line };
    return { cls: 'text-slate-700', text: line };
  }

  function statusIcon(status: string): { icon: typeof Check; color: string } {
    switch (status) {
      case 'configured': return { icon: Check, color: 'text-emerald-500' };
      case 'missing': return { icon: X, color: 'text-red-500' };
      case 'partial': return { icon: AlertTriangle, color: 'text-amber-500' };
      default: return { icon: AlertTriangle, color: 'text-slate-400' };
    }
  }

  function formatTimestamp(ts: string): string {
    try {
      const d = new Date(ts);
      return d.toLocaleString();
    } catch {
      return ts;
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
  }

  function groupByCategory(items: NetworkPrerequisiteCheck[]): Record<string, NetworkPrerequisiteCheck[]> {
    const groups: Record<string, NetworkPrerequisiteCheck[]> = {};
    for (const item of items) {
      const cat = item.category || 'other';
      if (!groups[cat]) groups[cat] = [];
      groups[cat].push(item);
    }
    return groups;
  }
</script>

<!-- ── Loading state ── -->
{#if loading}
  <div class="flex items-center justify-center py-16">
    <div class="text-center">
      <div class="w-8 h-8 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin mx-auto mb-3"></div>
      <p class="text-slate-500 text-sm">Loading configuration...</p>
    </div>
  </div>

{:else if error && !configContent}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <h3 class="text-lg font-semibold text-red-800">Unable to load configuration</h3>
    <p class="text-red-700 text-sm mt-1">{error}</p>
    <button
      type="button"
      onclick={() => void loadConfig()}
      class="inline-block mt-4 px-4 py-2 bg-red-600 text-white text-sm font-medium rounded-md hover:bg-red-700 transition-colors"
    >
      Retry
    </button>
  </div>

{:else}
  <div class="space-y-4">

    <!-- Banners -->
    {#if successBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-emerald-50 border border-emerald-200 text-emerald-800" role="status">
        <span>{successBanner}</span>
        <button type="button" onclick={() => (successBanner = '')} class="ml-4 text-emerald-600 hover:text-emerald-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}
    {#if errorBanner}
      <div class="flex items-center justify-between px-4 py-3 rounded-lg text-sm bg-red-50 border border-red-200 text-red-800" role="alert">
        <span>{errorBanner}</span>
        <button type="button" onclick={() => (errorBanner = '')} class="ml-4 text-red-600 hover:text-red-800" aria-label="Dismiss">
          <X class="w-4 h-4" />
        </button>
      </div>
    {/if}

    <!-- Header card -->
    {#if configMeta}
      <div class="bg-white rounded-lg shadow-sm border border-slate-200 px-6 py-4">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div class="flex items-center gap-3">
            <FileText class="w-5 h-5 text-slate-400" />
            <div>
              <p class="text-sm font-medium text-slate-900 font-mono">{configMeta.path}</p>
              <p class="text-xs text-slate-500">Last modified: {formatTimestamp(configMeta.lastModified)} · {formatBytes(configMeta.size)}</p>
            </div>
          </div>
          <div class="flex items-center gap-2">
            {#if issues.length === 0}
              <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-100 text-emerald-700">
                <Check class="w-3 h-3" />
                Valid
              </span>
            {:else}
              {@const errorCount = issues.filter(i => i.severity === 'error').length}
              {@const warnCount = issues.filter(i => i.severity === 'warning').length}
              {#if errorCount > 0}
                <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-red-100 text-red-700">
                  <X class="w-3 h-3" />
                  {errorCount} error{errorCount > 1 ? 's' : ''}
                </span>
              {/if}
              {#if warnCount > 0}
                <span class="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-medium bg-amber-100 text-amber-700">
                  <AlertTriangle class="w-3 h-3" />
                  {warnCount} warning{warnCount > 1 ? 's' : ''}
                </span>
              {/if}
            {/if}
          </div>
        </div>
      </div>
    {/if}

    <!-- Tab bar -->
    <div class="flex gap-1 border-b border-slate-200 mb-6" role="tablist">
      <button
        role="tab"
        aria-selected={activeTab === 'editor'}
        class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-t-md transition-colors {activeTab === 'editor' ? 'bg-white text-slate-900 border border-b-white border-slate-200 -mb-px' : 'text-slate-500 hover:text-slate-700'}"
        onclick={() => switchTab('editor')}
      >
        <FileText class="w-4 h-4" />
        Editor
      </button>
      <button
        role="tab"
        aria-selected={activeTab === 'backups'}
        class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-t-md transition-colors {activeTab === 'backups' ? 'bg-white text-slate-900 border border-b-white border-slate-200 -mb-px' : 'text-slate-500 hover:text-slate-700'}"
        onclick={() => switchTab('backups')}
      >
        <History class="w-4 h-4" />
        Backups
      </button>
      <button
        role="tab"
        aria-selected={activeTab === 'network'}
        class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-t-md transition-colors {activeTab === 'network' ? 'bg-white text-slate-900 border border-b-white border-slate-200 -mb-px' : 'text-slate-500 hover:text-slate-700'}"
        onclick={() => switchTab('network')}
      >
        <Network class="w-4 h-4" />
        Network
      </button>
    </div>

    <!-- ═══════════════ Editor Tab ═══════════════ -->
    {#if activeTab === 'editor'}
      <div class="space-y-4">
        <!-- Action toolbar -->
        {#if isOwner}
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onclick={handleValidate}
              disabled={validating}
              class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-slate-700 bg-white border border-slate-300 rounded-md hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <Check class="w-4 h-4" />
              {validating ? 'Validating...' : 'Validate'}
            </button>
            <button
              type="button"
              onclick={handlePreview}
              disabled={!hasChanges}
              class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-slate-700 bg-white border border-slate-300 rounded-md hover:bg-slate-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <FileDiff class="w-4 h-4" />
              Preview Diff
            </button>
            <button
              type="button"
              onclick={() => void requestApply()}
              disabled={!hasChanges || applying}
              class="inline-flex items-center gap-1.5 px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {applying ? 'Applying...' : 'Apply'}
            </button>
            {#if hasChanges}
              <button
                type="button"
                onclick={() => { editContent = configContent; issues = []; requiresRestart = false; restartReasons = []; lastValidatedContent = ''; }}
                class="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-slate-500 hover:text-slate-700 transition-colors"
              >
                <RotateCcw class="w-4 h-4" />
                Reset
              </button>
            {/if}
          </div>
        {/if}

        <!-- Config Editor -->
        <ConfigEditor
          bind:this={editorRef}
          bind:content={editContent}
          readonly={!isOwner}
          {issues}
          class="min-h-[400px]"
        />

        <ConfigIssueList
          {issues}
          onjump={(line) => editorRef?.scrollToLine(line)}
          onfix={handleAutoFix}
        />
      </div>

    <!-- ═══════════════ Backups Tab ═══════════════ -->
    {:else if activeTab === 'backups'}
      <div class="space-y-4">
        {#if loadingBackups}
          <div class="flex items-center justify-center py-12">
            <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          </div>
        {:else if backups.length === 0}
          <div class="bg-white rounded-lg shadow-sm border border-slate-200 p-8 text-center">
            <History class="w-10 h-10 text-slate-300 mx-auto mb-3" />
            <p class="text-sm text-slate-500">No backups yet. Backups are created when you apply config changes.</p>
          </div>
        {:else}
          <!-- Backup list -->
          <div class="bg-white rounded-lg shadow-sm border border-slate-200 overflow-hidden">
            <div class="px-4 py-3 border-b border-slate-100 bg-slate-50">
              <h3 class="text-sm font-semibold text-slate-700">Configuration Backups ({backups.length})</h3>
            </div>
            <div class="divide-y divide-slate-100">
              {#each backups as backup (backup.number)}
                <div class="px-4 py-3 flex items-center justify-between gap-3">
                  <div class="min-w-0">
                    <p class="text-sm font-medium text-slate-700">Backup #{backup.number}</p>
                    <p class="text-xs text-slate-500">{formatTimestamp(backup.timestamp)} · {formatBytes(backup.size)}</p>
                  </div>
                  <div class="flex items-center gap-1.5 flex-shrink-0">
                    <button
                      type="button"
                      onclick={() => viewBackup(backup.number, 'view')}
                      disabled={loadingBackupDetail}
                      class="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-slate-600 bg-white border border-slate-200 rounded hover:bg-slate-50 disabled:opacity-50 transition-colors"
                      title="View content"
                    >
                      <Eye class="w-3.5 h-3.5" />
                      View
                    </button>
                    <button
                      type="button"
                      onclick={() => viewBackup(backup.number, 'diff')}
                      disabled={loadingBackupDetail}
                      class="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-slate-600 bg-white border border-slate-200 rounded hover:bg-slate-50 disabled:opacity-50 transition-colors"
                      title="Diff vs current"
                    >
                      <FileDiff class="w-3.5 h-3.5" />
                      Diff
                    </button>
                    {#if isOwner}
                      <button
                        type="button"
                        onclick={() => requestRollback(backup.number)}
                        disabled={rolling}
                        class="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-white bg-red-600 rounded hover:bg-red-700 disabled:opacity-50 transition-colors"
                        title="Rollback to this backup"
                      >
                        <RotateCcw class="w-3.5 h-3.5" />
                        Rollback
                      </button>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          </div>

          <!-- Backup detail view -->
          {#if loadingBackupDetail}
            <div class="flex items-center justify-center py-8">
              <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
            </div>
          {:else if backupView}
            <div class="bg-white rounded-lg shadow-sm border border-slate-200 overflow-hidden">
              <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-slate-50">
                <h3 class="text-sm font-semibold text-slate-700">
                  Backup #{backupView.n} — {backupView.mode === 'view' ? 'Content' : 'Diff vs Current'}
                </h3>
                <button
                  type="button"
                  onclick={() => (backupView = null)}
                  class="text-slate-400 hover:text-slate-600 transition-colors"
                  aria-label="Close"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>
              <div class="p-4 overflow-auto max-h-[400px]">
                {#if backupView.mode === 'view'}
                  <pre class="font-mono text-xs text-slate-700 whitespace-pre-wrap">{backupView.content}</pre>
                {:else}
                  <div class="font-mono text-xs leading-relaxed">
                    {#each backupView.content.split('\n') as line}
                      {@const dl = renderDiffLine(line)}
                      <div class="px-2 py-0.5 {dl.cls}">{dl.text}</div>
                    {/each}
                  </div>
                {/if}
              </div>
            </div>
          {/if}
        {/if}
      </div>

    <!-- ═══════════════ Network Tab ═══════════════ -->
    {:else if activeTab === 'network'}
      <div class="space-y-4">
        {#if loadingNetwork}
          <div class="flex items-center justify-center py-12">
            <div class="w-6 h-6 border-2 border-slate-200 border-t-slate-500 rounded-full animate-spin"></div>
          </div>
        {:else if network && network.items.length > 0}
          {@const groups = groupByCategory(network.items)}
          {#each Object.entries(groups) as [category, items]}
            <div class="bg-white rounded-lg shadow-sm border border-slate-200 overflow-hidden">
              <div class="px-4 py-3 border-b border-slate-100 bg-slate-50">
                <h3 class="text-sm font-semibold text-slate-700 capitalize">{category}</h3>
              </div>
              <div class="divide-y divide-slate-100">
                {#each items as item}
                  {@const si = statusIcon(item.status)}
                  <div class="px-4 py-3">
                    <div class="flex items-start gap-3">
                      <si.icon class="w-4 h-4 mt-0.5 flex-shrink-0 {si.color}" />
                      <div class="flex-1 min-w-0">
                        <div class="flex items-center justify-between gap-2">
                          <p class="text-sm font-medium text-slate-700">{item.name}</p>
                          <span class="text-xs px-2 py-0.5 rounded-full font-medium {
                            item.status === 'configured' ? 'bg-emerald-100 text-emerald-700' :
                            item.status === 'missing' ? 'bg-red-100 text-red-700' :
                            item.status === 'partial' ? 'bg-amber-100 text-amber-700' :
                            'bg-slate-100 text-slate-600'
                          }">
                            {item.status}
                          </span>
                        </div>
                        <div class="mt-1 grid grid-cols-2 gap-x-4 text-xs">
                          <div>
                            <span class="text-slate-500">Expected:</span>
                            <span class="text-slate-700 ml-1">{item.expected}</span>
                          </div>
                          <div>
                            <span class="text-slate-500">Actual:</span>
                            <span class="text-slate-700 ml-1">{item.actual}</span>
                          </div>
                        </div>
                        {#if item.suggestion && item.status !== 'configured'}
                          <div class="mt-2 flex items-center gap-2">
                            <code class="text-xs bg-slate-100 text-slate-700 px-2 py-1 rounded font-mono flex-1 overflow-x-auto">{item.suggestion}</code>
                            <button
                              type="button"
                              onclick={() => copySuggestion(item.suggestion)}
                              class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium text-slate-600 bg-white border border-slate-200 rounded hover:bg-slate-50 flex-shrink-0 transition-colors"
                              title="Copy to clipboard"
                            >
                              <Copy class="w-3 h-3" />
                              {copiedSuggestion === item.suggestion ? 'Copied!' : 'Copy'}
                            </button>
                          </div>
                        {/if}
                      </div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/each}

          <!-- Refresh button -->
          <div class="flex justify-end">
            <button
              type="button"
              onclick={() => { networkLoaded = false; void loadNetwork(); }}
              disabled={loadingNetwork}
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-slate-600 bg-white border border-slate-200 rounded-md hover:bg-slate-50 disabled:opacity-50 transition-colors"
            >
              <RefreshCw class="w-4 h-4" />
              Refresh
            </button>
          </div>
        {:else if network && network.items.length === 0}
          <div class="bg-white rounded-lg shadow-sm border border-slate-200 p-8 text-center">
            <Network class="w-10 h-10 text-slate-300 mx-auto mb-3" />
            <p class="text-sm text-slate-500">No network prerequisite checks available.</p>
          </div>
        {:else}
          <div class="bg-white rounded-lg shadow-sm border border-slate-200 p-8 text-center">
            <Network class="w-10 h-10 text-slate-300 mx-auto mb-3" />
            <p class="text-sm text-slate-500">Network prerequisites could not be loaded.</p>
          </div>
        {/if}
      </div>
    {/if}
  </div>
{/if}

<!-- ═══════════════ Diff Modal ═══════════════ -->
{#if showDiffModal}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center"
    role="dialog"
    aria-modal="true"
    aria-labelledby="diff-modal-title"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div class="fixed inset-0 bg-black/50" onclick={() => (showDiffModal = false)}></div>
    <div class="relative bg-white rounded-lg shadow-xl max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
      <div class="p-6 border-b border-slate-200 flex items-center justify-between">
        <h3 id="diff-modal-title" class="text-lg font-semibold text-slate-900">Configuration Diff</h3>
        <button
          type="button"
          onclick={() => (showDiffModal = false)}
          class="text-slate-400 hover:text-slate-600 transition-colors"
          aria-label="Close"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
      <div class="p-6 overflow-auto flex-1">
        {#if diffText.trim() === ''}
          <p class="text-sm text-slate-500 text-center py-4">No differences found.</p>
        {:else}
          <div class="font-mono text-xs leading-relaxed">
            {#each diffText.split('\n') as line}
              {@const dl = renderDiffLine(line)}
              <div class="px-2 py-0.5 {dl.cls}">{dl.text}</div>
            {/each}
          </div>
        {/if}
      </div>
      <div class="p-4 border-t border-slate-200 flex justify-end gap-3">
        <button
          type="button"
          onclick={() => (showDiffModal = false)}
          class="px-4 py-2 text-sm font-medium text-slate-700 bg-white border border-slate-300 rounded-md hover:bg-slate-50 transition-colors"
        >
          Close
        </button>
        {#if isOwner}
          <button
            type="button"
            onclick={() => { showDiffModal = false; void requestApply(); }}
            class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 transition-colors"
          >
            Apply Changes
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<!-- ═══════════════ Apply Confirmation ═══════════════ -->
{#if showApplyConfirm}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center"
    role="dialog"
    aria-modal="true"
    aria-labelledby="apply-confirm-title"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div class="fixed inset-0 bg-black/50" onclick={() => (showApplyConfirm = false)}></div>
    <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
      <h2 id="apply-confirm-title" class="text-lg font-semibold text-slate-800">Apply configuration changes?</h2>
      <p class="text-sm text-slate-600 mt-2">
        This will overwrite the live ocserv configuration. A backup of the current config will be created before applying.
        The ocserv service will be reloaded to pick up the changes.
      </p>
      {#if diffText}
        <div class="mt-3 max-h-[200px] overflow-auto border border-slate-200 rounded-md">
          <div class="font-mono text-xs leading-relaxed p-2">
            {#each diffText.split('\n') as line}
              {@const dl = renderDiffLine(line)}
              <div class="px-1 {dl.cls}">{dl.text}</div>
            {/each}
          </div>
        </div>
      {/if}
      {#if requiresRestart}
        <div class="mt-3 p-3 bg-amber-50 border border-amber-200 rounded-md">
          <p class="text-sm font-medium text-amber-800">Warning: This change requires a full service restart.</p>
          <p class="text-xs text-amber-700 mt-1">Active VPN sessions will be disconnected.</p>
          {#if restartReasons.length > 0}
            <ul class="mt-2 text-xs text-amber-700 list-disc list-inside space-y-0.5">
              {#each restartReasons as reason}
                <li>{reason}</li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}
      <div class="flex items-center justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={() => (showApplyConfirm = false)}
          class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={handleApply}
          disabled={applying}
          class="px-4 py-2 text-sm font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {applying ? 'Applying...' : requiresRestart ? 'Restart & Apply' : 'Confirm Apply'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- ═══════════════ Rollback Confirmation ═══════════════ -->
{#if rollbackConfirmN !== null}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center"
    role="dialog"
    aria-modal="true"
    aria-labelledby="rollback-confirm-title"
  >
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div class="fixed inset-0 bg-black/50" onclick={() => (rollbackConfirmN = null)}></div>
    <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
      <h2 id="rollback-confirm-title" class="text-lg font-semibold text-slate-800">Rollback to backup #{rollbackConfirmN}?</h2>
      <p class="text-sm text-slate-600 mt-2">
        This will replace the current live configuration with backup #{rollbackConfirmN}.
        The ocserv service will be reloaded to pick up the restored configuration.
        This action cannot be undone.
      </p>
      <div class="flex items-center justify-end gap-3 mt-6">
        <button
          type="button"
          onclick={() => (rollbackConfirmN = null)}
          class="px-4 py-2 text-sm font-medium rounded-md border border-slate-200 text-slate-600 bg-white hover:bg-slate-50 transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onclick={handleRollback}
          disabled={rolling}
          class="px-4 py-2 text-sm font-medium rounded-md bg-red-600 text-white hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {rolling ? 'Rolling back...' : 'Confirm Rollback'}
        </button>
      </div>
    </div>
  </div>
{/if}
