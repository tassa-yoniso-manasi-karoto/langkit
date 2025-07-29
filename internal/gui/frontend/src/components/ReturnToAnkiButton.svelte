<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { logger } from '../lib/logger';
    import { isAnkiMode } from '../lib/runtime/bridge';
    import { hasConfig } from '../config';
    
    // State for runtime detection
    let inAnkiMode = false;
    let configLoaded = false;
    
    // State for the return hint drawer
    let showReturnHint = false;
    let hideHintTimeout: number;
    
    onMount(() => {
        // Function to show hint
        const showHintDelayed = () => {
            setTimeout(() => {
                showReturnHint = true;
                // Hide after 5 seconds
                hideHintTimeout = setTimeout(() => {
                    showReturnHint = false;
                }, 5000);
            }, 3000);
        };
        
        // Check for config and determine runtime mode
        if (hasConfig()) {
            configLoaded = true;
            inAnkiMode = isAnkiMode();
            logger.debug('ReturnToAnkiButton', 'Config loaded', { inAnkiMode });
            
            // Show hint if in Anki mode
            if (inAnkiMode) {
                showHintDelayed();
            }
        } else {
            // Wait a bit for config to load
            const checkInterval = setInterval(() => {
                if (hasConfig()) {
                    configLoaded = true;
                    inAnkiMode = isAnkiMode();
                    logger.debug('ReturnToAnkiButton', 'Config loaded after wait', { inAnkiMode });
                    clearInterval(checkInterval);
                    
                    // Show hint if in Anki mode
                    if (inAnkiMode) {
                        showHintDelayed();
                    }
                }
            }, 100);
            
            // Stop checking after 5 seconds
            setTimeout(() => clearInterval(checkInterval), 5000);
        }
    });
    
    onDestroy(() => {
        // Clear return hint timeout
        if (hideHintTimeout) {
            clearTimeout(hideHintTimeout);
        }
    });
    
    function handleReturnToAnki(e: MouseEvent) {
        e.preventDefault();
        e.stopPropagation();
        logger.info('ReturnToAnkiButton', 'Returning to Anki');
        
        // Call the global function injected by Python
        if ((window as any).returnToAnki) {
            (window as any).returnToAnki();
        } else {
            // Fallback: change title directly
            document.title = '__LANGKIT_RETURN_TO_ANKI__';
        }
    }
</script>

{#if configLoaded && inAnkiMode}
    <Portal target="body">
        <div class="fixed top-4 left-4" style="z-index: var(--z-index-return-to-anki);">
            <button
                class="w-10 h-10 flex items-center justify-center rounded-xl
                       bg-white/5 backdrop-blur-md border border-white/10
                       text-white/30 transition-all duration-300
                       hover:bg-white/10 hover:border-primary/30 hover:text-white/80
                       hover:-translate-y-0.5 hover:shadow-lg hover:shadow-primary/20
                       focus:outline-none focus:ring-2 focus:ring-primary/50"
                on:click={handleReturnToAnki}
                aria-label="Return to Anki"
            >
                <div class="pl-1" style="display: inline-block;">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" viewBox="0 0 16 16" fill="currentColor">
                        <path d="M12 10V8H7V6h5V4l3 3zm-1-1v4H6v3l-6-3V0h11v5h-1V1H2l4 2v9h4V9z"/>
                    </svg>
                </div>
            </button>
            
            <!-- Return hint drawer -->
            {#if showReturnHint}
                <div 
                    class="absolute left-full h-7 flex items-center overflow-hidden"
                    style="margin-left: -1px; top: 0.425rem;"
                >
                    <div 
                        class="h-full flex items-center px-4 bg-white/5 backdrop-blur-md 
                               border border-l-0 border-white/10 rounded-r-xl
                               text-white/50 text-sm whitespace-nowrap"
                        in:slide={{ axis: 'x', duration: 550, easing: cubicOut }}
                        out:slide={{ axis: 'x', duration: 450, easing: cubicOut }}
                    >
                        Press&nbsp;<span class="font-semibold text-white/70">ESC</span>&nbsp;to return to Anki
                    </div>
                </div>
            {/if}
        </div>
    </Portal>
{/if}