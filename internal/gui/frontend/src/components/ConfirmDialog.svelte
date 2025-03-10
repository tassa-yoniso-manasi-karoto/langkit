<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { fade } from 'svelte/transition';

    // A short message to display in the dialog:
    export let message: string = "Are you sure?";

    // Control whether the dialog is open:
    export let open: boolean = false;

    const dispatch = createEventDispatcher();

    /**
     * Closes the dialog with a specific outcome:
     *  - "confirm" if the user accepted
     *  - "cancel" otherwise
     */
    function closeDialog(action: 'confirm' | 'cancel') {
        dispatch(action);
    }

    /**
     * If the user clicks the backdrop, we treat it as cancel.
     * You could remove this if you don’t want backdrop-click to close.
     */
    function handleBackdropClick(e: MouseEvent) {
        // Only close if the click is on the backdrop, not on a child element.
        if (e.target === e.currentTarget) {
            closeDialog('cancel');
        }
    }
</script>

{#if open}
    <!-- Overlay -->
    <div
        class="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50"
        on:click={handleBackdropClick}
        transition:fade={{ duration: 150 }}
    >
        <!-- Modal Box -->
        <div
            class="bg-[#1e1e1e] rounded-xl border border-white/10 w-full max-w-sm p-6 shadow-2xl"
            transition:fade={{ duration: 200 }}
        >
            <!-- Title or icon (Optional) -->
            <h2 class="text-white text-lg font-semibold mb-2">
                Confirmation
            </h2>

            <!-- Main message -->
            <p class="text-gray-200 text-sm mb-6">
                {message}
            </p>

            <!-- Buttons row -->
            <div class="flex justify-end items-center gap-4">
                <button
                    class="px-4 py-2 font-medium rounded-md bg-white/10 text-white hover:bg-white/20 transition-colors"
                    on:click={() => closeDialog('cancel')}
                >
                    Cancel
                </button>
                <button
                    class="px-5 py-2 font-medium rounded-md bg-primary text-white hover:bg-primary/80 transition-colors"
                    on:click={() => closeDialog('confirm')}
                >
                    Confirm
                </button>
            </div>
        </div>
    </div>
{/if}

<style>
    /* 4 spaces for CSS indenting */
    .bg-primary {
        /* Example: your accent color #9f6ef7 if using your Tailwind theme */
        background-color: var(--accent-color, #9f6ef7);
    }
</style>
