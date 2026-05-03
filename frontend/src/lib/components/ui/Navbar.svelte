<script lang="ts">
  import { User } from 'lucide-svelte';

  type Props = {
    title: string;
    username: string;
    onLogout: () => void;
  };

  let { title, username, onLogout }: Props = $props();
  let showDropdown = $state(false);

  function toggleDropdown() {
    showDropdown = !showDropdown;
  }

  function handleLogout() {
    showDropdown = false;
    onLogout();
  }
</script>

<header class="h-16 bg-white border-b border-slate-200 flex items-center justify-between px-6 sticky top-0 z-20">
  <h2 class="text-lg font-semibold text-slate-700">{title}</h2>

  <div class="relative">
    <button
      type="button"
      onclick={toggleDropdown}
      class="flex items-center gap-2 text-sm text-slate-600 hover:text-slate-800 transition-colors"
    >
      <div class="w-8 h-8 rounded-full bg-slate-200 flex items-center justify-center">
        <User class="w-4 h-4 text-slate-600" />
      </div>
      <span class="hidden md:inline">{username}</span>
    </button>

    {#if showDropdown}
      <div class="absolute right-0 mt-2 w-48 bg-white rounded-lg shadow-lg border border-slate-200 py-1 z-50">
        <button
          type="button"
          onclick={handleLogout}
          class="w-full text-left px-4 py-2 text-sm text-slate-700 hover:bg-slate-50"
        >
          Sign out
        </button>
      </div>
    {/if}
  </div>
</header>
