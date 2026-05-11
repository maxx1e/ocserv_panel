<script lang="ts">
	import favicon from "$lib/assets/favicon.svg";
	import { ModeWatcher } from "mode-watcher";
	import { MetaTags } from "svelte-meta-tags";
	import { onNavigate } from "$app/navigation";
	import "../app.css";

	let { children } = $props();

	onNavigate((navigation) => {
		if (!document.startViewTransition) return;
		return new Promise((resolve) => {
			document.startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
		});
	});
</script>

<MetaTags
	title="SvelteForge Admin"
	titleTemplate="%s | SvelteForge Admin"
	description="A full-featured admin dashboard built with SvelteKit 2, Svelte 5, Tailwind CSS 4, Drizzle ORM, and session-based auth."
	openGraph={{
		type: "website",
		url: "https://svelteforge-admin.dev",
		title: "SvelteForge Admin",
		description:
			"A full-featured admin dashboard built with SvelteKit 2, Svelte 5, Tailwind CSS 4, Drizzle ORM, and session-based auth.",
		siteName: "SvelteForge Admin",
	}}
	twitter={{
		cardType: "summary",
		title: "SvelteForge Admin",
		description:
			"A full-featured admin dashboard built with SvelteKit 2, Svelte 5, Tailwind CSS 4, Drizzle ORM, and session-based auth.",
	}}
/>

<ModeWatcher />

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{@render children()}
