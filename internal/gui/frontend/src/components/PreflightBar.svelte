<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import type { ExpectationProfile } from '../api/generated/api.gen';
    import type { CheckState } from '../lib/checkResultStore';

    export let profiles: ExpectationProfile[] = [];
    export let checkMode: string = 'auto';
    export let quorum: number = 75;
    export let selectedProfileName: string = '';
    export let isProcessing: boolean = false;
    export let checkState: CheckState = 'unchecked';

    var dispatch = createEventDispatcher<{
        process: void;
        checkLibrary: void;
        modeChange: { mode: string; profileName: string; quorum: number };
    }>();

    // Build mode dropdown options from profiles
    interface ModeOption {
        value: string;
        label: string;
        mode: string;
        profileName: string;
    }

    $: modeOptions = buildModeOptions(profiles);

    function buildModeOptions(profs: ExpectationProfile[]): ModeOption[] {
        var opts: ModeOption[] = [
            { value: 'auto', label: 'Auto (Consistency)', mode: 'auto', profileName: '' },
            { value: 'from-settings', label: '(From Settings)', mode: 'profile', profileName: '' },
        ];
        for (var i = 0; i < profs.length; i++) {
            opts.push({
                value: 'profile:' + profs[i].name,
                label: 'Profile: ' + profs[i].name,
                mode: 'profile',
                profileName: profs[i].name,
            });
            opts.push({
                value: 'both:' + profs[i].name,
                label: 'Both: ' + profs[i].name,
                mode: 'both',
                profileName: profs[i].name,
            });
        }
        return opts;
    }

    // Derive select value from current mode + profile
    $: selectValue = deriveSelectValue(checkMode, selectedProfileName);

    function deriveSelectValue(mode: string, profName: string): string {
        if (mode === 'auto') return 'auto';
        if (!profName && mode === 'profile') return 'from-settings';
        if (mode === 'profile' && profName) return 'profile:' + profName;
        if (mode === 'both' && profName) return 'both:' + profName;
        return 'auto';
    }

    $: showQuorum = checkMode === 'auto' || checkMode === 'both';

    function handleModeChange(e: Event) {
        var value = (e.target as HTMLSelectElement).value;

        if (value === 'manage') {
            dispatch('checkLibrary');
            return;
        }

        var opt = modeOptions.find(function(o) { return o.value === value; });
        if (opt) {
            checkMode = opt.mode;
            selectedProfileName = opt.profileName;
            dispatch('modeChange', { mode: opt.mode, profileName: opt.profileName, quorum: quorum });
        }
    }

    function handleQuorumChange() {
        dispatch('modeChange', { mode: checkMode, profileName: selectedProfileName, quorum: quorum });
    }

    // Process button label based on check state
    $: processLabel = (function() {
        if (isProcessing) return 'Processing...';
        if (checkState === 'checked_with_errors_unacknowledged') return 'Acknowledge & Process';
        return 'Process Files';
    })();

    $: processIcon = (function() {
        if (isProcessing) return 'refresh';
        if (checkState === 'checked_with_errors_unacknowledged') return 'warning';
        return 'play_arrow';
    })();
</script>

<!-- Shell: ChatGPT fills in mode dropdown styling, inline quorum slider, action buttons layout -->
<div class="space-y-2">
    <!-- Row 1: Mode + Quorum -->
    <div class="flex items-center gap-3">
        <select
            class="flex-1 h-9 px-3 text-sm rounded-md bg-white/5 border border-white/10
                   text-white/90 focus:outline-none focus:border-primary/50 transition-colors"
            value={selectValue}
            on:change={handleModeChange}
        >
            {#each modeOptions as opt}
                <option value={opt.value} class="bg-[#1e1e1e] text-white">{opt.label}</option>
            {/each}
            <option disabled>───────────</option>
            <option value="manage" class="bg-[#1e1e1e] text-white">Manage Profiles...</option>
        </select>

        {#if showQuorum}
            <div class="flex items-center gap-2 shrink-0">
                <span class="text-xs text-white/50">Quorum</span>
                <input type="range" min="50" max="100" step="5"
                       bind:value={quorum}
                       on:change={handleQuorumChange}
                       class="w-20 h-1 accent-primary" />
                <span class="text-xs text-white/70 w-8 text-right">{quorum}%</span>
            </div>
        {/if}
    </div>

    <!-- Row 2: Action buttons -->
    <div class="flex items-center gap-3">
        <button
            class="px-4 py-2 text-sm rounded-md border border-white/10 bg-white/5
                   text-white/70 hover:bg-white/10 transition-colors"
            on:click={() => dispatch('checkLibrary')}
        >
            <span class="material-icons text-sm align-middle mr-1">search</span>
            Check Library
        </button>

        <button
            class="flex-1 px-4 py-2.5 text-sm font-medium rounded-md transition-colors
                   {checkState === 'checked_with_errors_unacknowledged'
                       ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30 hover:bg-amber-500/30'
                       : 'bg-primary/20 text-primary border border-primary/30 hover:bg-primary/30'}
                   disabled:opacity-40 disabled:cursor-not-allowed"
            disabled={isProcessing}
            on:click={() => dispatch('process')}
        >
            <span class="material-icons text-sm align-middle mr-1
                         {isProcessing ? 'animate-spin' : ''}">{processIcon}</span>
            {processLabel}
        </button>
    </div>
</div>
