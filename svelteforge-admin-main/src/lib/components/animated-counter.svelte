<script lang="ts">
	let {
		value,
		duration = 800,
		formatFn = (n: number) => n.toLocaleString(),
	}: {
		value: number;
		duration?: number;
		formatFn?: (n: number) => string;
	} = $props();

	let display = $state("0");

	$effect(() => {
		const target = value;
		const start = performance.now();
		let frame: number;

		function easeOutExpo(t: number): number {
			return t === 1 ? 1 : 1 - Math.pow(2, -10 * t);
		}

		function animate(now: number) {
			const elapsed = now - start;
			const progress = Math.min(elapsed / duration, 1);
			const eased = easeOutExpo(progress);
			const current = Math.round(eased * target);
			display = formatFn(current);

			if (progress < 1) {
				frame = requestAnimationFrame(animate);
			}
		}

		frame = requestAnimationFrame(animate);
		return () => cancelAnimationFrame(frame);
	});
</script>

{display}
