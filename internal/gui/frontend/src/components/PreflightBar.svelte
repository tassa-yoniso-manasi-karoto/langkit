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

    $: isAcknowledge = checkState === 'checked_with_errors_unacknowledged';
    $: busy = isProcessing || checkState === 'running';
</script>

<div class={(isLite ? 'bg-white/10' : 'bg-white/5 backdrop-blur-md') + ' rounded-xl border border-white/10 p-2.5 space-y-2'}>
    <!-- Row 1: Mode dropdown + Quorum -->
    <div class="flex items-center gap-2">
        <select
            class="flex-1 h-9 px-3 text-sm rounded-lg bg-white/5 border border-white/10
                   text-white/90 focus:outline-none focus:border-primary/50 transition-colors"
            value={selectValue}
            on:change={handleModeChange}
        >
            {#each modeOptions as opt}
                <option value={opt.value} class="bg-bgold-800 text-white">{opt.label}</option>
            {/each}
            <option disabled>───────────</option>
            <option value="manage" class="bg-bgold-800 text-white">Manage Profiles...</option>
        </select>

        {#if showQuorum}
            <div class="shrink-0 flex items-center gap-1.5 h-9 px-2.5 rounded-lg
                        border border-white/10 bg-white/5 text-[11px] text-white/55">
                <span>Q:</span>
                <input type="range" min="50" max="100" step="5"
                       bind:value={quorum}
                       on:change={handleQuorumChange}
                       class="w-12 h-[3px] accent-primary" />
                <span class="font-mono text-xs text-white/80 font-medium">{quorum}%</span>
            </div>
        {/if}
    </div>

    <!-- Row 2: Check Media + Process -->
    <div class="flex items-center gap-2">
        <button
            class="h-10 px-3 text-sm rounded-lg border border-white/10 bg-white/5
                   text-white/70 hover:bg-white/10 hover:text-white/85 transition-colors shrink-0"
            on:click={() => dispatch('checkLibrary', { intent: 'run' })}
        >
            <span class="material-icons text-sm align-middle mr-1">fact_check</span>
            Check Media
        </button>

        <button
            class={'flex-1 h-10 px-4 text-sm font-semibold rounded-lg border transition-all '
                + 'flex items-center justify-center gap-1.5 '
                + (isAcknowledge
                    ? 'border-amber-400/30 bg-amber-500/[0.12] text-amber-200 hover:bg-amber-500/20 btn-process-ack'
                    : 'border-transparent bg-primary text-white btn-process')
                + (busy ? ' opacity-50 cursor-not-allowed' : '')}
            disabled={busy}
            on:click={() => dispatch('process')}
        >
            <span class={'material-icons text-sm' + (busy ? ' animate-spin' : '')}>
                {processIcon}
            </span>
            {processLabel}
        </button>
    </div>
</div>

<style>
    .btn-process {
        box-shadow: 0 4px 16px hsla(261, 90%, 50%, 0.25);
    }
    .btn-process:hover:not(:disabled) {
        box-shadow: 0 4px 20px hsla(261, 90%, 50%, 0.35);
    }
    .btn-process-ack {
        box-shadow: 0 4px 16px hsla(40, 90%, 50%, 0.15);
    }
</style>
