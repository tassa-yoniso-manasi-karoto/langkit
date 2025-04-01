<script>
	import Portal from "svelte-portal/src/Portal.svelte";
	import { onMount } from "svelte";

	// The tooltip content can be passed as a prop or via the default slot.
	export let message = "";
	// Position of the tooltip: top, right, bottom, left
	export let position = "top";

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
			let styleString = "";
			
			// Position the tooltip based on the specified position
			switch(position) {
				case "right":
					styleString = `
						left: ${rect.right + 10}px;
						top: ${rect.top + rect.height / 2}px;
						transform: translateY(-50%);
					`;
					break;
				case "bottom":
					styleString = `
						left: ${rect.left + rect.width / 2}px;
						top: ${rect.bottom + 10}px;
						transform: translateX(-50%);
					`;
					break;
				case "left":
					styleString = `
						right: ${window.innerWidth - rect.left + 10}px;
						top: ${rect.top + rect.height / 2}px;
						transform: translateY(-50%);
					`;
					break;
				case "top":
				default:
					styleString = `
						left: ${rect.left + rect.width / 2}px;
						top: ${rect.top - 8}px;
						transform: translate(-50%, -100%);
					`;
			}
			
			tooltipStyle = styleString;
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

<!-- Render the tooltip in a portal with the globally defined z-index class -->
{#if visible}
	<Portal target="body">
		<div bind:this={tooltip} style={tooltipStyle} class="fixed hovertip">
			<div class="bg-gray-800 text-white text-xs rounded-lg p-2 max-w-64 shadow-lg relative">
				{#if message}
					<!-- Use Tailwind's whitespace-pre-line to respect newline characters -->
					<div class="whitespace-pre-line">
						{message}
					</div>
				{:else}
					<slot />
				{/if}
				<!-- Tooltip arrow - position based on tooltip position -->
				{#if position === 'top' || position === 'bottom'}
					<div class="absolute {position === 'top' ? 'bottom-0 translate-y-full' : 'top-0 -translate-y-full'} left-1/2 transform -translate-x-1/2 w-2 h-2 bg-gray-800 rotate-45"></div>
				{:else}
					<div class="absolute {position === 'left' ? 'right-0 translate-x-full' : 'left-0 -translate-x-full'} top-1/2 transform -translate-y-1/2 w-2 h-2 bg-gray-800 rotate-45"></div>
				{/if}
			</div>
		</div>
	</Portal>
{/if}