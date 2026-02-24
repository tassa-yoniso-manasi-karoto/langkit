<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { liteModeStore } from '../lib/stores';
    import type { ExpectationProfile } from '../api/generated/api.gen';
    import type { CheckState } from '../lib/checkResultStore';

    export let profiles: ExpectationProfile[] = [];
    export let checkMode: string = 'auto';
    export let quorum: number = 75;
    export let selectedProfileName: string = '';
    export let isProcessing: boolean = false;
    export let checkState: CheckState = 'unchecked';
    $: isLite = $liteModeStore.enabled;

    var dispatch = createEventDispatcher<{
        process: void;
        checkLibrary: { intent: 'run' | 'manage' };
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
            dispatch('checkLibrary', { intent: 'manage' });
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
        if (checkState === 'running') return 'Checking...';
        if (isProcessing) return 'Processing...';
        if (checkState === 'checked_with_errors_unacknowledged') return 'Acknowledge & Process';
        return 'Process Files';
    })();

    $: processIcon = (function() {
        if (checkState === 'running') return 'refresh';
        if (isProcessing) return 'refresh';
        if (checkState === 'checked_with_errors_unacknowledged') return 'warning';
        return 'play_arrow';
    })();

    $: processTone = (function() {
        if (checkState === 'checked_with_errors_unacknowledged') {
            return 'bg-amber-500/20 text-amber-200 border border-amber-400/35 hover:bg-amber-500/30';
        }
        if (checkState === 'running') {
            return 'bg-primary/20 text-primary border border-primary/35';
        }
        return 'bg-primary/20 text-primary border border-primary/30 hover:bg-primary/30';
    })();

    $: busy = isProcessing || checkState === 'running';

    $: statusLabel = (function() {
        if (checkState === 'unchecked') return 'Not checked';
        if (checkState === 'stale') return 'Stale results';
        if (checkState === 'running') return 'Checking';
        if (checkState === 'checked_clean') return 'Clean';
        if (checkState === 'checked_with_errors_unacknowledged') return 'Errors found';
        if (checkState === 'checked_with_errors_acknowledged') return 'Acknowledged';
        return 'Unknown';
    })();

    $: statusClass = (function() {
        if (checkState === 'checked_clean') return 'border-green-400/35 bg-green-500/15 text-green-200';
        if (checkState === 'checked_with_errors_unacknowledged') return 'border-amber-400/35 bg-amber-500/15 text-amber-200';
        if (checkState === 'running') return 'border-primary/35 bg-primary/15 text-primary';
        if (checkState === 'stale') return 'border-amber-400/35 bg-amber-500/10 text-amber-200';
        return 'border-white/15 bg-white/10 text-white/65';
    })();
</script>

<div class={(isLite ? 'bg-white/10' : 'bg-white/5 backdrop-blur-md') + ' rounded-xl border border-white/10 p-2.5 space-y-2'}>
    <div class="flex items-center justify-between gap-2">
        <div class="text-[11px] text-white/55">Preflight</div>
        <span class="text-[10px] px-2 py-0.5 rounded-full border {statusClass}">
            {statusLabel}
        </span>
    </div>

    <div class="flex items-center gap-2">
        <select
            class="flex-1 h-9 px-3 text-sm rounded-md bg-white/5 border border-white/10
                   text-white/90 focus:outline-none focus:border-primary/50 transition-colors"
            value={selectValue}
            on:change={handleModeChange}
        >
            {#each modeOptions as opt}
                <option value={opt.value} class="bg-bgold-800 text-white">{opt.label}</option>
            {/each}
            <option disabled>───────────</option>
            <option value="manage" class="bg-bgold-800 text-white">⚙ Manage Profiles...</option>
        </select>

        {#if showQuorum}
            <div class="shrink-0 rounded-md border border-white/10 bg-white/5 px-2 py-1.5 min-w-[110px]">
                <div class="flex items-center justify-between text-[10px] text-white/55">
                    <span>Quorum</span>
                    <span class="text-white/80">{quorum}%</span>
                </div>
                <input type="range"
                       min="50"
                       max="100"
                       step="5"
                       bind:value={quorum}
                       on:change={handleQuorumChange}
                       class="w-full h-1 accent-primary mt-1" />
            </div>
        {/if}
    </div>

    <div class="grid grid-cols-[auto_minmax(0,1fr)] gap-2">
        <button
            class="h-10 px-3 text-sm rounded-md border border-white/10 bg-white/5
                   text-white/70 hover:bg-white/10 hover:text-white/85 transition-colors"
            on:click={() => dispatch('checkLibrary', { intent: 'run' })}
        >
            <span class="material-icons text-sm align-middle mr-1">search</span>
            Check Library
        </button>

        <button
            class="h-10 px-4 text-sm font-medium rounded-md transition-colors
                   {processTone}
                   disabled:opacity-45 disabled:cursor-not-allowed"
            disabled={busy}
            on:click={() => dispatch('process')}
        >
            <span class="material-icons text-sm align-middle mr-1 {busy ? 'animate-spin' : ''}">
                {processIcon}
            </span>
            {processLabel}
        </button>
    </div>
</div>
