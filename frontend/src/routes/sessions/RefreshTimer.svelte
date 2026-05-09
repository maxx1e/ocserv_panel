<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { RefreshCw, ChevronDown } from 'lucide-svelte';

  type Props = {
    onRefresh: () => void;
    paused?: boolean;
  };

  let { onRefresh, paused = false }: Props = $props();

  const STORAGE_KEY = 'sessions_refresh_interval';
  const INTERVALS = [
    { value: 10000, label: '10s' },
    { value: 15000, label: '15s' },
    { value: 20000, label: '20s' },
    { value: 30000, label: '30s' },
    { value: 60000, label: '1m' },
  ];

  let selectedInterval = $state(30000);
  let timeRemaining = $state(30000);
  let dropdownOpen = $state(false);
  let tickId: ReturnType<typeof setInterval> | null = null;

  // Derived
  const progress = $derived(1 - timeRemaining / selectedInterval);
  const selectedLabel = $derived(INTERVALS.find(i => i.value === selectedInterval)?.label ?? '30s');

  // SVG circle constants
  const RADIUS = 9;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;
  const strokeDashoffset = $derived(CIRCUMFERENCE * (1 - progress));

  function loadInterval() {
    if (typeof window === 'undefined') return;
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const val = parseInt(stored, 10);
      if (INTERVALS.some(i => i.value === val)) {
        selectedInterval = val;
        timeRemaining = val;
      }
    }
  }

  function saveInterval(val: number) {
    localStorage.setItem(STORAGE_KEY, String(val));
  }

  function selectInterval(val: number) {
    selectedInterval = val;
    timeRemaining = val;
    saveInterval(val);
    dropdownOpen = false;
    restartTimer();
  }

  function startTimer() {
    stopTimer();
    tickId = setInterval(() => {
      if (paused) return;
      timeRemaining -= 1000;
      if (timeRemaining <= 0) {
        timeRemaining = selectedInterval;
        onRefresh();
      }
    }, 1000);
  }

  function stopTimer() {
    if (tickId) { clearInterval(tickId); tickId = null; }
  }

  function restartTimer() {
    stopTimer();
    startTimer();
  }

  function handleManualRefresh() {
    timeRemaining = selectedInterval;
    onRefresh();
    restartTimer();
  }

  onMount(() => {
    loadInterval();
    timeRemaining = selectedInterval;
    startTimer();
  });

  onDestroy(() => {
    stopTimer();
  });

  // Pause/resume effect
  $effect(() => {
    if (paused) {
      stopTimer();
    } else if (!tickId) {
      startTimer();
    }
  });
</script>

<div class="flex items-center gap-2">
  <!-- Countdown ring + interval selector -->
  <div class="relative">
    <button
      type="button"
      onclick={() => (dropdownOpen = !dropdownOpen)}
      class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
      aria-label="Change refresh interval"
      aria-expanded={dropdownOpen}
    >
      <!-- SVG Ring -->
      <svg width="20" height="20" viewBox="0 0 22 22" class="flex-shrink-0">
        <!-- Background circle -->
        <circle cx="11" cy="11" r={RADIUS} fill="none" stroke="#e2e8f0" stroke-width="2" />
        <!-- Progress circle -->
        <circle
          cx="11" cy="11" r={RADIUS}
          fill="none"
          stroke="#64748b"
          stroke-width="2"
          stroke-linecap="round"
          stroke-dasharray={CIRCUMFERENCE}
          stroke-dashoffset={strokeDashoffset}
          transform="rotate(-90 11 11)"
          class="transition-[stroke-dashoffset] duration-1000 ease-linear"
        />
      </svg>
      <span class="text-xs text-slate-500">{selectedLabel}</span>
      <ChevronDown class="w-3 h-3 text-slate-400" />
    </button>

    <!-- Dropdown -->
    {#if dropdownOpen}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="absolute top-full left-0 mt-1 bg-white border border-slate-200 rounded-md shadow-lg z-50 py-1 min-w-[80px]"
        role="menu"
      >
        {#each INTERVALS as option}
          <button
            type="button"
            role="menuitem"
            onclick={() => selectInterval(option.value)}
            class="w-full text-left px-3 py-1.5 text-sm hover:bg-slate-50 transition-colors
              {option.value === selectedInterval ? 'text-slate-800 font-medium bg-slate-50' : 'text-slate-600'}"
          >
            {option.label}
          </button>
        {/each}
      </div>
      <!-- Backdrop to close dropdown -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="fixed inset-0 z-40"
        onclick={() => (dropdownOpen = false)}
        onkeydown={(e) => { if (e.key === 'Escape') dropdownOpen = false; }}
      ></div>
    {/if}
  </div>

  <!-- Refresh button -->
  <button
    type="button"
    onclick={handleManualRefresh}
    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
    aria-label="Refresh now"
  >
    <RefreshCw class="w-4 h-4" />
    Refresh
  </button>
</div>
