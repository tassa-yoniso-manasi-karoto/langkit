<script>
	import Portal from "svelte-portal/src/Portal.svelte";
	import { onMount } from "svelte";

	// The tooltip content can be passed as a prop or via the default slot.
	export let message = "";

	// Controls tooltip visibility.
	let visible = false;

	// References to the trigger element and the tooltip element.
	let trigger;
	let tooltip;

	// This string will hold inline styles to position the tooltip.
	let tooltipStyle = "";

	// Called when the mouse enters the trigger element.
	function showTooltip() {
		visible = true;
		updatePosition();
	}

	// Called when the mouse leaves the trigger element.
	function hideTooltip() {
		visible = false;
	}

	// Compute the tooltip's position relative to the trigger element.
	function updatePosition() {
		if (trigger) {
			const rect = trigger.getBoundingClientRect();
			// Position the tooltip above the trigger:
			// - left: centered horizontally over the trigger.
			// - top: placed a few pixels above the trigger.
			tooltipStyle = `
				left: ${rect.left + rect.width / 2}px;
				top: ${rect.top - 8}px;
				transform: translate(-50%, -100%);
			`;
		}
	}

	// Update the tooltip's position on window resize.
	onMount(() => {
		window.addEventListener("resize", updatePosition);
		return () => {
			window.removeEventListener("resize", updatePosition);
		};
	});
</script>

<!-- The trigger element -->
<div bind:this={trigger} on:mouseenter={showTooltip} on:mouseleave={hideTooltip}>
	<slot name="trigger" />
</div>

<!-- Render the tooltip in a portal so it isn't clipped by parent overflow settings -->
{#if visible}
	<Portal target="body">
		<div bind:this={tooltip} style={tooltipStyle} class="fixed z-50">
			<div class="bg-gray-800 text-white text-xs rounded-lg p-2 max-w-64 shadow-lg relative">
				{#if message}
					<!-- Use Tailwind's whitespace-pre-line to respect newline characters -->
					<div class="whitespace-pre-line">
						{message}
					</div>
				{:else}
					<slot />
				{/if}
				<!-- Tooltip arrow -->
				<div class="absolute bottom-0 left-1/2 transform -translate-x-1/2 translate-y-full w-2 h-2 bg-gray-800 rotate-45"></div>
			</div>
		</div>
	</Portal>
{/if}
