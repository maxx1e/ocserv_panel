<script lang="ts">
  import HealthCard from '$lib/components/ui/HealthCard.svelte';
  import HealthSummaryStrip from '$lib/components/ui/HealthSummaryStrip.svelte';
  import type { HealthItem } from '$lib/components/ui/HealthCard.svelte';
  import type { SystemHealth } from '$lib/api';

  type Props = {
    health: SystemHealth | null;
    healthLoading: boolean;
    healthError: string;
    healthCheckedAt: Date | null;
    storageItems: HealthItem[];
  };

  let { health, healthLoading, healthError, healthCheckedAt, storageItems }: Props = $props();

  const HEALTH_TOOLTIPS: Record<string, Record<string, { title: string; description: string; action: string }>> = {
    systemd: {
      warning: { title: 'Systemd Service', description: 'ocserv service is inactive or not running.', action: 'Start the service via systemctl or the panel controls.' },
      critical: { title: 'Systemd Service', description: 'ocserv systemd unit is in failed state.', action: 'Check journalctl logs and restart the service.' },
    },
    socket: {
      warning: { title: 'Socket / Process', description: 'ocserv socket not present. Process may not be fully started.', action: 'Check systemd unit status and socket permissions.' },
    },
    config: {
      warning: { title: 'Config File', description: 'ocserv configuration file not found at expected path.', action: 'Verify config path in Settings > Installation.' },
      critical: { title: 'Config File', description: 'Configuration present but failed validation.', action: 'Fix config errors and run validation again.' },
    },
    database: {
      critical: { title: 'Database', description: 'SQLite DB unavailable, locked, or not writable.', action: 'Check file permissions, disk space, and run migrations.' },
    },
    panelConfig: {
      warning: { title: 'Panel Config', description: 'Panel configuration file not found.', action: 'Verify config path or create default configuration.' },
    },
    passwordFile: {
      warning: { title: 'Password File', description: 'VPN password file not found. User operations will fail.', action: 'Create a VPN user to auto-generate the file, or check the configured path in Settings.' },
    },
  };

  const vpnServerItems = $derived<HealthItem[]>(
    healthError ? [{ label: 'Health check unavailable', status: 'unknown' as HealthItem['status'], detail: healthError }] :
    !health ? [] : (() => {
      const v = health.vpnServer;
      const svcStatus: HealthItem['status'] = v.systemdStatus === 'active' ? 'healthy'
        : v.systemdStatus === 'failed' ? 'critical'
        : v.systemdStatus === 'inactive' ? 'warning'
        : 'unknown';
      const socketStatus: HealthItem['status'] = v.socketPresent ? 'healthy' : 'warning';
      const cfgStatus: HealthItem['status'] = v.configPresent && v.configValid ? 'healthy'
        : v.configPresent && !v.configValid ? 'critical' : 'warning';
      const pwdFileStatus: HealthItem['status'] = v.passwordFilePresent ? 'healthy' : 'warning';
      return [
        {
          label: 'Systemd service',
          status: svcStatus,
          detail: svcStatus !== 'healthy' ? `Status: ${v.systemdStatus}` : undefined,
          tooltip: svcStatus !== 'healthy' ? HEALTH_TOOLTIPS.systemd[svcStatus] ?? null : null,
        },
        {
          label: 'Socket / process',
          status: socketStatus,
          tooltip: socketStatus !== 'healthy' ? HEALTH_TOOLTIPS.socket[socketStatus] ?? null : null,
        },
        {
          label: 'Config file',
          status: cfgStatus,
          detail: v.configErrors || undefined,
          tooltip: cfgStatus !== 'healthy' ? HEALTH_TOOLTIPS.config[cfgStatus] ?? null : null,
        },
        {
          label: 'Password file',
          status: pwdFileStatus,
          tooltip: pwdFileStatus !== 'healthy' ? HEALTH_TOOLTIPS.passwordFile[pwdFileStatus] ?? null : null,
        },
      ];
    })()
  );

  const panelItems = $derived<HealthItem[]>(
    healthError ? [{ label: 'Health check unavailable', status: 'unknown' as HealthItem['status'], detail: healthError }] :
    !health ? [] : [
      { label: 'Backend', status: 'healthy' as HealthItem['status'] },
      {
        label: 'Database',
        status: (health.panel.dbReachable ? 'healthy' : 'critical') as HealthItem['status'],
        tooltip: !health.panel.dbReachable ? HEALTH_TOOLTIPS.database.critical : null,
      },
      {
        label: 'Config file',
        status: (health.panel.configPresent ? 'healthy' : 'warning') as HealthItem['status'],
        tooltip: !health.panel.configPresent ? HEALTH_TOOLTIPS.panelConfig.warning : null,
      },
      { label: 'WebUI users', status: 'healthy' as HealthItem['status'], detail: `${health.panel.webuiUserCount} user${health.panel.webuiUserCount === 1 ? '' : 's'}` },
    ]
  );

  const allHealthItems = $derived<HealthItem[]>([...vpnServerItems, ...panelItems, ...storageItems]);
</script>

<!-- Health summary strip -->
{#if allHealthItems.length > 0}
  <HealthSummaryStrip items={allHealthItems} />
{/if}

<!-- Health cards -->
<div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
  <HealthCard
    title="VPN Server"
    items={vpnServerItems}
    lastChecked={healthCheckedAt}
    loading={healthLoading && !health}
  />
  <HealthCard
    title="WebUI Panel"
    items={panelItems}
    lastChecked={healthCheckedAt}
    loading={healthLoading && !health}
  />
</div>
