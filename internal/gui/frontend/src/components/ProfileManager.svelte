<script lang="ts">
    import { onMount } from 'svelte';
    import { slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { settings, liteModeStore } from '../lib/stores';
    import { expectationProfilesStore } from '../lib/expectationProfilesStore';
    import type { ExpectationProfile } from '../api/services/expectation';
    import { logger } from '../lib/logger';

    import TextInput from './TextInput.svelte';
    import ConfirmDialog from './ConfirmDialog.svelte';

    // Sentinel values that cannot be used as profile names
    const SENTINEL_FROM_SETTINGS = '\x00from-settings';
    const SENTINEL_NEW_PROFILE = '\x00new-profile';

    export let selectedProfileName: string = '';

    let profiles: ExpectationProfile[] = [];
    let isEditing = false;
    let isCreatingNew = false;
    let showDeleteConfirm = false;

    // The immutable name of the currently loaded profile (for delete/rename)
    let loadedProfileName = '';

    // Fields to preserve on save for properties not exposed in the UI
    let preservedCheckExternalAudio = false;
    let preservedVideoExtensions: string[] = [];

    // Editing fields
    let editName = '';
    let editAudioLangs = '';
    let editSubLangs = '';
    let editRequireVideoTrack = true;
    let editRequireLanguageTags = true;
    let editDurationTolerance = 2.0;
    let editSubtitleLineThreshold = 80.0;

    $: isLite = $liteModeStore.enabled;

    // Subscribe to profiles store
    var unsub = expectationProfilesStore.subscribe(function(p) {
        profiles = p;
    });

    onMount(function() {
        expectationProfilesStore.ensureLoaded().catch(function(e) {
            logger.error('profileManager', 'Failed to load profiles on mount', { error: e });
        });
        return unsub;
    });

    // Build dropdown options: "From Settings" + saved profiles + "+ New"
    $: dropdownOptions = buildDropdownOptions(profiles);

    interface DropdownOption {
        value: string;
        label: string;
    }

    function buildDropdownOptions(profs: ExpectationProfile[]): DropdownOption[] {
        var opts: DropdownOption[] = [
            { value: SENTINEL_FROM_SETTINGS, label: '(From Settings)' }
        ];
        for (var i = 0; i < profs.length; i++) {
            opts.push({ value: profs[i].name, label: profs[i].name });
        }
        opts.push({ value: SENTINEL_NEW_PROFILE, label: '+ New Profile' });
        return opts;
    }

    // Map selectedProfileName to the select value
    $: selectValue = selectedProfileName || SENTINEL_FROM_SETTINGS;

    function handleDropdownChange(e: Event) {
        var value = (e.target as HTMLSelectElement).value;

        if (value === SENTINEL_NEW_PROFILE) {
            startNewProfile();
            return;
        }

        if (value === SENTINEL_FROM_SETTINGS) {
            selectedProfileName = '';
            isCreatingNew = false;
            loadedProfileName = '';
            loadFromSettings();
        } else {
            selectedProfileName = value;
            isCreatingNew = false;
            loadProfile(value);
        }
        isEditing = true;
    }

    function loadFromSettings() {
        var s = $settings;
        editName = '';
        loadedProfileName = '';
        preservedCheckExternalAudio = false;
        preservedVideoExtensions = [];
        editAudioLangs = s.targetLanguage || '';
        var subParts: string[] = [];
        if (s.targetLanguage) subParts.push(s.targetLanguage);
        if (s.nativeLanguages) {
            var natives = s.nativeLanguages.split(',');
            for (var i = 0; i < natives.length; i++) {
                var trimmed = natives[i].trim();
                if (trimmed && subParts.indexOf(trimmed) === -1) {
                    subParts.push(trimmed);
                }
            }
        }
        editSubLangs = subParts.join(', ');
        editRequireVideoTrack = true;
        editRequireLanguageTags = true;
        editDurationTolerance = 2.0;
        editSubtitleLineThreshold = 80.0;
    }

    function loadProfile(name: string) {
        var found: ExpectationProfile | null = null;
        for (var i = 0; i < profiles.length; i++) {
            if (profiles[i].name === name) {
                found = profiles[i];
                break;
            }
        }
        if (!found) return;

        loadedProfileName = found.name;
        preservedCheckExternalAudio = found.checkExternalAudioFiles;
        preservedVideoExtensions = found.videoExtensions || [];

        editName = found.name;
        editAudioLangs = (found.expectedAudioLangs || []).join(', ');
        editSubLangs = (found.expectedSubtitleLangs || []).join(', ');
        editRequireVideoTrack = found.requireVideoTrack;
        editRequireLanguageTags = found.requireLanguageTags;
        editDurationTolerance = found.durationTolerancePercent ?? 2.0;
        editSubtitleLineThreshold = found.subtitleLineThresholdPct ?? 80.0;
    }

    function startNewProfile() {
        isCreatingNew = true;
        isEditing = true;
        selectedProfileName = '';
        loadedProfileName = '';
        preservedCheckExternalAudio = false;
        preservedVideoExtensions = [];
        editName = '';
        editAudioLangs = '';
        editSubLangs = '';
        editRequireVideoTrack = true;
        editRequireLanguageTags = true;
        editDurationTolerance = 2.0;
        editSubtitleLineThreshold = 80.0;
    }

    function parseLangList(str: string): string[] {
        if (!str.trim()) return [];
        var parts = str.split(',');
        var result: string[] = [];
        for (var i = 0; i < parts.length; i++) {
            var trimmed = parts[i].trim();
            if (trimmed) result.push(trimmed);
        }
        return result;
    }

    function isReservedName(name: string): boolean {
        var lower = name.toLowerCase();
        return lower === '(from settings)' ||
               lower === '+ new profile' ||
               name === SENTINEL_FROM_SETTINGS ||
               name === SENTINEL_NEW_PROFILE;
    }

    async function handleSave() {
        var newName = editName.trim();
        if (!newName) {
            logger.error('profileManager', 'Profile name cannot be empty');
            return;
        }

        if (isReservedName(newName)) {
            logger.error('profileManager', 'Profile name is reserved: ' + newName);
            return;
        }

        var profile: ExpectationProfile = {
            name: newName,
            expectedAudioLangs: parseLangList(editAudioLangs),
            expectedSubtitleLangs: parseLangList(editSubLangs),
            requireVideoTrack: editRequireVideoTrack,
            requireLanguageTags: editRequireLanguageTags,
            durationTolerancePercent: editDurationTolerance,
            subtitleLineThresholdPct: editSubtitleLineThreshold,
            checkExternalAudioFiles: preservedCheckExternalAudio,
            videoExtensions: preservedVideoExtensions,
        };

        try {
            // Handle rename: if the name changed, delete the old profile first
            if (loadedProfileName && loadedProfileName !== newName) {
                await expectationProfilesStore.remove(loadedProfileName);
            }

            await expectationProfilesStore.save(profile);
            selectedProfileName = newName;
            loadedProfileName = newName;
            isCreatingNew = false;
        } catch (e) {
            logger.error('profileManager', 'Failed to save profile', { error: e });
        }
    }

    function requestDelete() {
        // Always delete the originally loaded profile, not the edited name
        if (!loadedProfileName) return;
        showDeleteConfirm = true;
    }

    async function handleDeleteConfirm() {
        showDeleteConfirm = false;
        try {
            await expectationProfilesStore.remove(loadedProfileName);
            selectedProfileName = '';
            loadedProfileName = '';
            isEditing = false;
            isCreatingNew = false;
            loadFromSettings();
        } catch (e) {
            logger.error('profileManager', 'Failed to delete profile', { error: e });
        }
    }
</script>

<div class="space-y-3">
    <!-- Profile Selector -->
    <div class="flex items-center gap-2">
        <select
            class="flex-1 h-[42px] px-3 text-sm font-medium rounded-md
                   bg-white/5 border-2 border-white/10 text-white/90
                   focus:outline-none focus:border-primary/50
                   transition-colors"
            value={selectValue}
            on:change={handleDropdownChange}
        >
            {#each dropdownOptions as opt}
                <option value={opt.value} class="bg-[#1e1e1e] text-white">{opt.label}</option>
            {/each}
        </select>

        {#if loadedProfileName && !isCreatingNew}
            <button
                class="h-[42px] w-[42px] flex items-center justify-center rounded-md
                       bg-red-500/10 border border-red-500/20 text-red-400
                       hover:bg-red-500/20 transition-colors"
                on:click={requestDelete}
                title="Delete profile"
            >
                <span class="material-icons text-base">delete</span>
            </button>
        {/if}
    </div>

    <!-- Inline Editor -->
    {#if isEditing}
        <div class="space-y-2.5 p-3 rounded-lg bg-white/5 border border-white/10"
             transition:slide={{ duration: isLite ? 0 : 200, easing: cubicOut }}>

            {#if isCreatingNew || loadedProfileName}
                <div>
                    <label class="block text-xs text-white/50 mb-1">Profile Name</label>
                    <TextInput
                        bind:value={editName}
                        placeholder="e.g. Thai anime"
                        className="h-8 text-xs"
                    />
                </div>
            {/if}

            <div>
                <label class="block text-xs text-white/50 mb-1">Expected Audio Languages</label>
                <TextInput
                    bind:value={editAudioLangs}
                    placeholder="e.g. ja, th"
                    className="h-8 text-xs"
                />
            </div>

            <div>
                <label class="block text-xs text-white/50 mb-1">Expected Subtitle Languages</label>
                <TextInput
                    bind:value={editSubLangs}
                    placeholder="e.g. ja, th, en"
                    className="h-8 text-xs"
                />
            </div>

            <div class="grid grid-cols-2 gap-2">
                <label class="flex items-center gap-2 text-xs text-white/70 cursor-pointer">
                    <input type="checkbox" bind:checked={editRequireVideoTrack}
                           class="rounded border-white/20 bg-white/5 text-primary focus:ring-primary/30" />
                    Require video track
                </label>
                <label class="flex items-center gap-2 text-xs text-white/70 cursor-pointer">
                    <input type="checkbox" bind:checked={editRequireLanguageTags}
                           class="rounded border-white/20 bg-white/5 text-primary focus:ring-primary/30" />
                    Require language tags
                </label>
            </div>

            <div class="grid grid-cols-2 gap-2">
                <div>
                    <label class="block text-xs text-white/50 mb-1">Duration tolerance (%)</label>
                    <input type="number" bind:value={editDurationTolerance}
                           min="0" max="50" step="0.5"
                           class="w-full h-8 px-2 text-xs rounded-md bg-white/5 border border-white/10 text-white/90
                                  focus:outline-none focus:border-primary/50 transition-colors" />
                </div>
                <div>
                    <label class="block text-xs text-white/50 mb-1">Subtitle threshold (%)</label>
                    <input type="number" bind:value={editSubtitleLineThreshold}
                           min="0" max="100" step="5"
                           class="w-full h-8 px-2 text-xs rounded-md bg-white/5 border border-white/10 text-white/90
                                  focus:outline-none focus:border-primary/50 transition-colors" />
                </div>
            </div>

            <!-- Save button (only for named profiles) -->
            {#if isCreatingNew || loadedProfileName}
                <div class="flex justify-end pt-1">
                    <button
                        class="px-4 py-1.5 text-xs font-medium rounded-md
                               bg-primary/20 text-primary border border-primary/30
                               hover:bg-primary/30 transition-colors
                               disabled:opacity-40 disabled:cursor-not-allowed"
                        disabled={!editName.trim() || isReservedName(editName.trim())}
                        on:click={handleSave}
                    >
                        {isCreatingNew ? 'Create' : 'Save'}
                    </button>
                </div>
            {/if}
        </div>
    {/if}
</div>

<ConfirmDialog
    open={showDeleteConfirm}
    message={'Delete profile "' + loadedProfileName + '"?'}
    on:confirm={handleDeleteConfirm}
    on:cancel={() => showDeleteConfirm = false}
/>
