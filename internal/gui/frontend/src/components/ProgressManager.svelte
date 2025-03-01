<script lang="ts">
    import { onMount } from 'svelte';
    import { slide } from 'svelte/transition';
    import { quintOut } from 'svelte/easing';
    import { progressBars, removeProgressBar, type ProgressBarData } from '../lib/progressBarsStore';

    // If user wants to collapse the bar list
    let isCollapsed: boolean = false;

    // Automatic removal of fully completed bars (not in error) after 2s
    onMount(() => {
        const unsub = progressBars.subscribe((bars) => {
            for (const bar of bars) {
                if (bar.progress >= 100 && bar.color !== 'red') {
                    setTimeout(() => removeProgressBar(bar.id), 2000);
                }
            }
        });
        return () => unsub();
    });

    function toggleCollapse() {
        isCollapsed = !isCollapsed;
    }

    // Sort in descending order of priority
    // (highest priority => top of list, lowest => bottom)
    $: sortedBars = $progressBars
        .slice()
        .sort((a, b) => (b.priority ?? 50) - (a.priority ?? 50));
</script>

<!-- Only show if there's at least one bar. -->
{#if $progressBars.length > 0}
    <div class="flex flex-col w-full bg-[#1a1a1a] text-white border-t border-[#2a2a2a] px-3 py-2 space-y-2 overflow-hidden">

        <!-- A minimal top row with a label & action buttons -->
        <div class="flex items-center justify-between">
            <span class="font-bold text-base text-gray-300">
                Processing Status
            </span>
            <div class="flex items-center gap-2">
                <!-- Collapse/Expand toggle button -->
                <button 
                    class="flex items-center justify-center w-4 h-4
                           text-white/50 hover:text-white/80
                           transition-colors duration-200
                           focus:outline-none"
                    on:click={toggleCollapse}
                    aria-label="{isCollapsed ? 'Expand' : 'Collapse'} progress bars"
                >
                    <span class="material-icons text-[12px]">
                        {isCollapsed ? 'expand_more' : 'expand_less'}
                    </span>
                </button>
                
                <!-- Clear all progress bars button -->
                <button 
                    class="flex items-center justify-center w-4 h-4
                           text-red-400/50 hover:text-red-400
                           transition-colors duration-200
                           focus:outline-none"
                    on:click={() => {
                        $progressBars.forEach(bar => removeProgressBar(bar.id));
                    }}
                    aria-label="Clear all progress bars"
                >
                    <span class="material-icons text-[12px]">close</span>
                </button>
            </div>
        </div>

        <!-- Drawer-like animation for progress bars -->
        {#if !isCollapsed}
            <div 
                transition:slide={{ duration: 200, easing: quintOut }} 
                class="space-y-1"
            >
                {#each sortedBars as bar (bar.id)}
                    <!-- Single bar row -->
                    <div class="flex flex-col gap-0.5 p-1 rounded">
                        <div class="flex items-center justify-between text-sm text-gray-200">
                            <span class="truncate">
                                {bar.operation}{#if bar.description} - {bar.description}{/if}
                            </span>
                            <span>
                                ({bar.current}/{bar.total}) {Math.round(bar.progress)}%
                            </span>
                        </div>
                        <div class="relative w-full bg-[#333] rounded-full overflow-hidden {bar.size || 'h-2.5'}">
                            <div
                                class="absolute inset-0 transition-all duration-200 bg-accent"
                                style="width: {bar.progress}%;"
                            />
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
{/if}
