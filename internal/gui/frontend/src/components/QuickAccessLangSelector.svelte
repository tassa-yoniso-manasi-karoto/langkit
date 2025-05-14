<script lang="ts">
    import { slide } from 'svelte/transition';
    import { createEventDispatcher } from 'svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    
    // Props
    export let languageTag = '';
    export let isValidLanguage: boolean | null = null;
    export let isChecking = false;
    export let validationError = '';
    export let showAudioTrackIndex = false;
    export let audioTrackIndex = 0;
    
    // Hovertip content for track override
    const trackOverrideHovertip = "In case the audiotracks of your media files don't have proper languages tags, set the number/index of the audio track to use as basis for processing here. \n\n It is still a good idea to set the language tag for STT, romanization... etc.";
    
    const dispatch = createEventDispatcher();
    
    function toggleAudioTrackIndex() {
        showAudioTrackIndex = !showAudioTrackIndex;
        if (!showAudioTrackIndex) {
            audioTrackIndex = 0; // Reset to 0 when hiding
        } else {
            audioTrackIndex = audioTrackIndex === 0 ? 1 : audioTrackIndex;
        }
        
        dispatch('audioTrackChange', { showAudioTrackIndex, audioTrackIndex });
    }
    
    function updateLanguageTag(e: Event) {
        const input = e.target as HTMLInputElement;
        dispatch('languageTagChange', { languageTag: input.value });
    }
    
    function handleAudioTrackChange() {
        dispatch('audioTrackChange', { showAudioTrackIndex, audioTrackIndex });
    }
</script>

<div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
    <!-- First column: Target Language section -->
    <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
        <!-- Label that will shrink to nothing if needed -->
        <div class="overflow-hidden">
            <span class="text-unobtrusive text-sm truncate block">
                Target Language
            </span>
        </div>
        
        <!-- Input field with fixed width -->
        <div class="relative">
            <input
                type="text"
                bind:value={languageTag}
                maxlength="9"
                placeholder="e.g. ja, zh-Hans"
                class="w-24 bg-ui-element backdrop-blur-sm border border-primary/30 rounded px-2 py-2
                       hover:bg-ui-element-hover hover:border-primary/50 focus:border-primary focus:ring-1 focus:ring-primary
                       focus:outline-none transition-colors duration-200 text-xs font-bold shadow-sm shadow-primary/10"
                on:input={updateLanguageTag}
            />
            {#if isChecking}
                <span class="absolute right-2 top-1.5 material-icons animate-spin text-primary/70 text-sm" style="font-size: 1.4rem;">
                    refresh
                </span>
            {:else if isValidLanguage === false}
                <span class="absolute right-2 top-1.5 material-icons text-red-500 text-sm" style="font-size: 1.4rem;"
                      title={validationError}>
                    error
                </span>
            {:else if isValidLanguage === true}
                <span class="absolute right-2 top-1.5 material-icons text-pale-green text-sm" style="font-size: 1.4rem;">
                    check_circle
                </span>
            {/if}
        </div>
    </div>

    <!-- Second column: Audio track button with panel -->
    <div class="audio-track-container flex items-center overflow-hidden">
        <!-- Disclosure arrow button -->
        <button
            class="flex items-center justify-center p-4 w-6 h-6
                   border border-primary/30 
                   hover:border-primary/60 hover:bg-ui-element-hover 
                   transition-all duration-500 focus:outline-none
                   {showAudioTrackIndex 
                      ? 'bg-ui-element rounded-tl rounded-bl rounded-tr-none rounded-br-none shadow-sm shadow-primary/20'
                      : 'bg-input-bg rounded shadow-sm shadow-primary/10'}"
            on:click={toggleAudioTrackIndex}
            title="Toggle method used to select audio track"
        >
            <span class="transform transition-transform duration-1000 text-primary/80
                         hover:text-primary text-2xl leading-none"
                  class:rotate-180={showAudioTrackIndex}>
                🡸
            </span>
        </button>

        <!-- Audio track panel - with inline transition -->
        {#if showAudioTrackIndex}
            <div class="-ml-px flex items-center"
                 transition:slide={{ duration: 200, axis: 'x' }}>
                <!-- Panel with reduced content for better space usage -->
                <div class="flex items-center bg-ui-element backdrop-blur-sm
                           border border-primary/30 border-l-0
                           rounded-r px-2 p-4 h-6 shadow-sm shadow-primary/10">
                    <div class="overflow-hidden min-w-0">
                        <span class="text-primary text-sm truncate block whitespace-nowrap">
                            Track Override
                        </span>
                    </div>
                    <Hovertip message={trackOverrideHovertip}>
                        <span slot="trigger" class="material-icons material-icon-adjust flex-shrink-0 text-primary cursor-help pl-1 pr-1 p-1.5 leading-none text-sm">
                            help_outline
                        </span>
                    </Hovertip>
                    <!-- Compact input field -->
                    <NumericInput
                        bind:value={audioTrackIndex}
                        min={1}
                        max={99}
                        fullWidth={false}
                        className="!w-10 h-6 px-1 py-0 text-xs border focus:ring-1"
                        on:change={handleAudioTrackChange}
                    />
                </div>
            </div>
        {/if}
    </div>
</div>

<style>
    /* Make the audio track container take up only the space it needs */
    .audio-track-container {
        width: fit-content;
        white-space: nowrap;
        /* z-index moved to app.css */
    }
    
    /* Add subtle glow on hover for the inputs */
    input:hover {
        box-shadow: 0 0 8px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }
    
    input:focus {
        box-shadow: 0 0 12px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
</style>