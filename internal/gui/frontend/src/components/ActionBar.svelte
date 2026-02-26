<script lang="ts">
    import { createEventDispatcher, onDestroy } from 'svelte';
    import { liteModeStore } from '../lib/stores';
    import type { ExpectationProfile } from '../api/generated/api.gen';
    import type { CheckState } from '../lib/checkResultStore';
    import { invalidationErrorStore } from '../lib/invalidationErrorStore';
    import ProcessErrorTooltip from './ProcessErrorTooltip.svelte';

    export let profiles: ExpectationProfile[] = [];
    export let checkMode: string = 'auto';
    export let selectedProfileName: string = '';
    export let isProcessing: boolean = false;
    export let checkState: CheckState = 'unchecked';
    $: isLite = $liteModeStore.enabled;

    var dispatch = createEventDispatcher<{
        process: void;
        checkLibrary: { intent: 'run' | 'manage' };
        modeChange: { mode: string; profileName: string };
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
            dispatch('modeChange', { mode: opt.mode, profileName: opt.profileName });
        }
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

    // Invalidation error gating (restores old ProcessButton behavior)
    var errors: any[] = [];
    var showTooltip = false;
    var processButtonRef: HTMLButtonElement;
    var tooltipPosition = { x: 0, y: 0 };

    var unsubErrors = invalidationErrorStore.subscribe(function(val) {
        errors = val;
    });
    onDestroy(function() { unsubErrors(); });

    $: hasCriticalErrors = errors.some(function(e) {
        return e.severity === 'critical';
    });
    $: shouldGrayOut = busy || errors.some(function(e) {
        return (e.id === 'no-media' || e.id === 'no-features')
            && e.severity === 'critical';
    });
    $: hasAnyErrors = errors.length > 0;

    function handleProcessClick() {
        if (hasCriticalErrors || busy) return;
        dispatch('process');
    }

    function handleProcessMouseOver(event: MouseEvent) {
        if (!hasAnyErrors) return;
        showTooltip = true;
        tooltipPosition = {
            x: event.clientX,
            y: event.clientY - 10
        };
    }

    function handleProcessMouseMove(event: MouseEvent) {
        if (!showTooltip || !hasAnyErrors) return;
        tooltipPosition = {
            x: event.clientX,
            y: event.clientY - 10
        };
    }

    function handleProcessMouseOut() {
        showTooltip = false;
    }
</script>

<div class="flex items-center gap-2">
    <select
        class="w-44 shrink-0 h-12 px-3 text-sm rounded-lg bg-white/5 border border-white/10
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

    <button
        class="h-12 px-3 text-sm rounded-lg border border-white/10 bg-white/5
               text-white/70 hover:bg-white/10 hover:text-white/85 transition-colors shrink-0
               flex items-center gap-1.5"
        on:click={() => dispatch('checkLibrary', { intent: 'run' })}
    >
        <svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" class="shrink-0"><path fill="currentColor" d="m19.352 7.617l-3.96-3.563c-1.127-1.015-1.69-1.523-2.383-1.788L13 5c0 2.357 0 3.536.732 4.268S15.643 10 18 10h3.58c-.362-.704-1.012-1.288-2.228-2.383M14 22h-4c-3.771 0-5.657 0-6.828-1.172c-.447-.446-.723-.995-.894-1.713c-.119-.5-.179-.75-.064-1.042s.368-.461.874-.799l.626-.417a2.32 2.32 0 0 1 2.572 0a3.09 3.09 0 0 0 3.428 0a2.32 2.32 0 0 1 2.572 0a3.09 3.09 0 0 0 3.428 0a2.32 2.32 0 0 1 2.572 0a3.09 3.09 0 0 0 3.428 0a.162.162 0 0 1 .251.143c-.078 1.872-.328 3.02-1.137 3.828C19.657 22 17.771 22 14 22"/><path fill="currentColor" d="M2 10c0-3.771 0-5.657 1.172-6.828S6.239 2 10.03 2c.606 0 1.091 0 1.5.017q-.02.12-.02.244l-.01 2.834c0 1.097 0 2.067.105 2.848c.114.847.375 1.694 1.067 2.386c.69.69 1.538.952 2.385 1.066c.781.105 1.751.105 2.848.105h4.088v.827a.62.62 0 0 1-.279.53a3.09 3.09 0 0 1-3.428 0a2.32 2.32 0 0 0-2.572 0a3.09 3.09 0 0 1-3.428 0a2.32 2.32 0 0 0-2.572 0a3.09 3.09 0 0 1-3.428 0a2.32 2.32 0 0 0-2.572 0l-.16.107c-.684.456-1.026.684-1.29.543S2 12.954 2 12.132z"/></svg>
        Check Files
    </button>

    <button
        bind:this={processButtonRef}
        class={'flex-1 h-12 px-5 text-sm font-semibold rounded-lg border transition-all '
            + 'flex items-center justify-center gap-1.5 whitespace-nowrap '
            + (isAcknowledge
                ? 'border-amber-400/30 bg-amber-500/[0.12] text-amber-200 hover:bg-amber-500/20 btn-process-ack'
                : 'border-transparent bg-primary text-white btn-process')
            + (shouldGrayOut ? ' opacity-50 cursor-not-allowed' : '')}
        disabled={busy || hasCriticalErrors}
        on:click={handleProcessClick}
        on:mouseover={handleProcessMouseOver}
        on:mousemove={handleProcessMouseMove}
        on:mouseout={handleProcessMouseOut}
    >
        <span class={'material-icons text-sm' + (busy ? ' animate-spin' : '')}>
            {processIcon}
        </span>
        {processLabel}
    </button>

    {#if showTooltip && hasAnyErrors}
        <ProcessErrorTooltip position={tooltipPosition} />
    {/if}
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
