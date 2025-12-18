<script lang="ts">
    import { fade, fly } from 'svelte/transition';
    import { cubicOut, quintOut } from 'svelte/easing';
    import { onMount, onDestroy, createEventDispatcher } from 'svelte';
    import { liteModeStore } from '../lib/stores';
    import { CheckUpgrade, GetChangelog, MarkVersionSeen } from '../api/services/changelog';
    import type { ChangelogEntry, UpgradeInfo } from '../api/generated/api.gen';
    import { logger } from '../lib/logger';

    // Props
    export let triggerCheck = false;  // Set to true to trigger upgrade check

    // Internal state
    let visible = false;
    let version = '';
    let upgradeType = '';
    let changelogEntries: ChangelogEntry[] = [];
    let loading = false;
    let error = '';

    const dispatch = createEventDispatcher();

    // Track lite mode for Qt+Windows compatibility
    $: liteMode = $liteModeStore.enabled;

    // Check if this is a major release - either the upgrade itself is major,
    // or any entry in the cumulative changelog is a major version (X.0.0)
    $: containsMajorRelease = changelogEntries.some(entry => {
        const match = entry.version.match(/^(\d+)\.0\.0$/);
        return match !== null;
    });
    $: isMajorRelease = upgradeType === 'major' || containsMajorRelease;

    // Organize changelog entries into sections
    $: changelog = organizeChangelog(changelogEntries);

    function organizeChangelog(entries: ChangelogEntry[]): { added: string[]; changed: string[]; fixed: string[]; deprecated: string[]; removed: string[]; security: string[] } {
        const result = {
            added: [] as string[],
            changed: [] as string[],
            fixed: [] as string[],
            deprecated: [] as string[],
            removed: [] as string[],
            security: [] as string[]
        };

        for (const entry of entries) {
            for (const section of entry.sections) {
                const title = section.title.toLowerCase();
                if (title === 'added') {
                    result.added.push(...section.items);
                } else if (title === 'changed') {
                    result.changed.push(...section.items);
                } else if (title === 'fixed') {
                    result.fixed.push(...section.items);
                } else if (title === 'deprecated') {
                    result.deprecated.push(...section.items);
                } else if (title === 'removed') {
                    result.removed.push(...section.items);
                } else if (title === 'security') {
                    result.security.push(...section.items);
                }
            }
        }

        return result;
    }

    // Check for upgrade when triggered
    async function checkForUpgrade() {
        if (loading) return;

        loading = true;
        error = '';

        try {
            logger.debug('Changelog', 'Checking for upgrade...');
            const info: UpgradeInfo = await CheckUpgrade();

            logger.debug('Changelog', 'Upgrade check result', {
                hasUpgrade: info.hasUpgrade,
                previousVersion: info.previousVersion,
                currentVersion: info.currentVersion,
                upgradeType: info.upgradeType,
                shouldShowChangelog: info.shouldShowChangelog
            });

            if (info.shouldShowChangelog) {
                version = info.currentVersion;
                upgradeType = info.upgradeType;

                // Fetch changelog entries since last seen version
                const changelogResponse = await GetChangelog(info.previousVersion || undefined);
                changelogEntries = changelogResponse.entries;

                logger.info('Changelog', 'Showing changelog popup', {
                    version: version,
                    entriesCount: changelogEntries.length
                });

                visible = true;
            } else {
                logger.debug('Changelog', 'No changelog to show');
                dispatch('checked', { shouldShow: false });
            }
        } catch (err) {
            logger.error('Changelog', 'Failed to check for upgrade', { error: err });
            error = err instanceof Error ? err.message : 'Unknown error';
            dispatch('checked', { shouldShow: false, error: error });
        } finally {
            loading = false;
        }
    }

    // Watch for triggerCheck changes
    $: if (triggerCheck) {
        checkForUpgrade();
    }

    // Dismiss handler
    async function handleDismiss() {
        visible = false;

        // Mark version as seen
        try {
            await MarkVersionSeen();
            logger.info('Changelog', 'Marked version as seen', { version });
        } catch (err) {
            logger.error('Changelog', 'Failed to mark version as seen', { error: err });
        }

        dispatch('dismissed');
    }

    // Keyboard handler for Escape key
    function handleKeydown(e: KeyboardEvent) {
        if (e.key === 'Escape' && visible) {
            handleDismiss();
        }
    }

    // Click outside to dismiss
    function handleBackdropClick(e: MouseEvent) {
        if (e.target === e.currentTarget) {
            handleDismiss();
        }
    }

    onMount(() => {
        window.addEventListener('keydown', handleKeydown);
    });

    onDestroy(() => {
        window.removeEventListener('keydown', handleKeydown);
    });
</script>

{#if visible}
    <!-- Backdrop overlay -->
    <div
        class="changelog-backdrop fixed inset-0 flex items-center justify-center p-4"
        class:backdrop-blur-sm={!liteMode}
        on:click={handleBackdropClick}
        in:fade={{ duration: 300 }}
        out:fade={{ duration: 200 }}
    >
        <!-- Card container - flies in from right -->
        <div
            class="changelog-card relative w-full max-w-lg overflow-hidden rounded-2xl"
            in:fly={{ x: 80, duration: 500, easing: quintOut, opacity: 0 }}
            out:fly={{ x: 40, duration: 250, easing: cubicOut, opacity: 0 }}
        >
            <!-- Teal gradient accent bar at top (sparse use of teal) -->
            <div class="accent-bar"></div>

            <!-- Glass/solid background layer -->
            <div
                class="card-background absolute inset-0"
                class:card-background--lite={liteMode}
            ></div>

            <!-- Content layer -->
            <div class="relative">
                <!-- Header -->
                <div class="px-6 pt-5 pb-4">
                    <div class="flex items-center gap-3">
                        <!-- Version badge -->
                        <span class="version-badge">
                            v{version}
                        </span>
                        {#if isMajorRelease}
                            <span class="major-release-label">
                                {#each 'MAJOR RELEASE'.split('') as char, i}
                                    <span class="typewriter-char" style="animation-delay: {0.3 + i * 0.18}s">{char === ' ' ? '\u00A0' : char}</span>
                                {/each}
                                <span class="typewriter-caret"></span>
                            </span>
                        {/if}
                        <h2 class="text-xl font-semibold text-white tracking-tight">What's New</h2>
                    </div>
                </div>

                <!-- Scrollable content area -->
                <div class="changelog-content px-6 pb-2">
                    <!-- Added section -->
                    {#if changelog.added.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--added">
                                <span class="material-icons">add_circle</span>
                                <h3>Added</h3>
                            </div>
                            <ul class="section-list section-list--added">
                                {#each changelog.added as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}

                    <!-- Changed section -->
                    {#if changelog.changed.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--changed">
                                <span class="material-icons">autorenew</span>
                                <h3>Changed</h3>
                            </div>
                            <ul class="section-list">
                                {#each changelog.changed as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}

                    <!-- Fixed section -->
                    {#if changelog.fixed.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--fixed">
                                <span class="material-icons">build_circle</span>
                                <h3>Fixed</h3>
                            </div>
                            <ul class="section-list">
                                {#each changelog.fixed as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}

                    <!-- Deprecated section -->
                    {#if changelog.deprecated.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--deprecated">
                                <span class="material-icons">warning</span>
                                <h3>Deprecated</h3>
                            </div>
                            <ul class="section-list">
                                {#each changelog.deprecated as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}

                    <!-- Removed section -->
                    {#if changelog.removed.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--removed">
                                <span class="material-icons">remove_circle</span>
                                <h3>Removed</h3>
                            </div>
                            <ul class="section-list">
                                {#each changelog.removed as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}

                    <!-- Security section -->
                    {#if changelog.security.length > 0}
                        <div class="changelog-section">
                            <div class="section-header section-header--security">
                                <span class="material-icons">security</span>
                                <h3>Security</h3>
                            </div>
                            <ul class="section-list">
                                {#each changelog.security as item}
                                    <li>{item}</li>
                                {/each}
                            </ul>
                        </div>
                    {/if}
                </div>

                <!-- Footer with dismiss button -->
                <div class="px-6 py-4 border-t border-white/5">
                    <button
                        class="dismiss-button"
                        on:click={handleDismiss}
                    >
                        Got it
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .changelog-backdrop {
        background-color: rgba(0, 0, 0, 0.55);
        z-index: 9995;
    }

    .changelog-card {
        max-height: calc(100vh - 4rem);
        display: flex;
        flex-direction: column;
        box-shadow:
            0 25px 50px -12px rgba(0, 0, 0, 0.6),
            0 0 0 1px rgba(255, 255, 255, 0.08);
    }

    /* Teal gradient accent bar - simple left to right, muted */
    .accent-bar {
        height: 3px;
        width: 100%;
        background: linear-gradient(
            90deg,
            hsla(var(--accent-hue), var(--accent-saturation), var(--accent-lightness), 0.7),
            hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) - 15%), 0.8)
        );
        position: relative;
        z-index: 1;
    }

    .card-background {
        background-color: rgba(22, 22, 30, 0.88);
        backdrop-filter: blur(24px);
        -webkit-backdrop-filter: blur(24px);
    }

    .card-background--lite {
        background-color: rgba(26, 26, 34, 0.98);
        backdrop-filter: none;
        -webkit-backdrop-filter: none;
    }

    /* Version badge - accent color (teal) */
    .version-badge {
        padding: 0.25rem 0.75rem;
        font-size: 0.75rem;
        font-weight: 600;
        border-radius: 9999px;
        background-color: hsla(var(--accent-hue), var(--accent-saturation), var(--accent-lightness), 0.15);
        color: hsl(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 15%));
        border: 1px solid hsla(var(--accent-hue), var(--accent-saturation), var(--accent-lightness), 0.3);
        letter-spacing: 0.02em;
    }

    /* Major release label container - dark green */
    .major-release-label {
        font-family: "Roboto Slab", serif;
        font-size: 1.25rem;
        font-weight: 700;
        color: hsl(142, 60%, 45%);
        text-transform: uppercase;
        white-space: nowrap;
        display: inline-flex;
    }

    /* Individual typewriter characters */
    .typewriter-char {
        display: inline-block;
        max-width: 0;
        overflow: hidden;
        vertical-align: bottom;
        animation: char-appear 0.01s forwards;
    }

    @keyframes char-appear {
        from { max-width: 0; }
        to { max-width: 1em; }
    }

    /* Blinking caret that disappears after typing completes */
    .typewriter-caret {
        width: 2px;
        background-color: hsl(142, 60%, 45%);
        margin-left: 2px;
        align-self: stretch;
        animation: blink-caret 1s ease-in-out 2.8, caret-fade 0.5s 2.8s forwards;
    }

    @keyframes blink-caret {
        0%, 100% { opacity: 0.9; }
        50% { opacity: 0.3; }
    }

    @keyframes caret-fade {
        to { opacity: 0; visibility: hidden; }
    }

    .changelog-content {
        max-height: 50vh;
        overflow-y: auto;
        scrollbar-width: thin;
        scrollbar-color: rgba(255, 255, 255, 0.15) transparent;
    }

    .changelog-content::-webkit-scrollbar {
        width: 6px;
    }

    .changelog-content::-webkit-scrollbar-track {
        background: transparent;
    }

    .changelog-content::-webkit-scrollbar-thumb {
        background-color: rgba(255, 255, 255, 0.15);
        border-radius: 3px;
    }

    .changelog-section {
        margin-bottom: 1.25rem;
    }

    .changelog-section:last-child {
        margin-bottom: 0.5rem;
    }

    .section-header {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-bottom: 0.625rem;
    }

    .section-header :global(.material-icons) {
        font-size: 1.125rem;
    }

    .section-header h3 {
        font-size: 0.9rem;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.05em;
    }

    /* Section color variants */
    .section-header--added {
        color: hsl(142, 70%, 55%);
    }

    .section-header--changed {
        color: hsl(210, 90%, 65%);
    }

    .section-header--fixed {
        color: hsl(45, 95%, 60%);
    }

    .section-header--deprecated {
        color: hsl(30, 90%, 60%);
    }

    .section-header--removed {
        color: hsl(0, 70%, 60%);
    }

    .section-header--security {
        color: hsl(280, 70%, 65%);
    }

    /* Fixed alignment: left-aligned list with proper bullet positioning */
    .section-list {
        margin: 0;
        padding-left: 1.5rem;
        list-style-type: disc;
        text-align: left;
    }

    .section-list li {
        font-size: 0.875rem;
        line-height: 1.6;
        color: rgba(255, 255, 255, 0.85);
        margin-bottom: 0.5rem;
        text-align: left;
        padding-left: 0.25rem;
    }

    /* Added section items are semi-bold */
    .section-list--added li {
        font-weight: 600;
        color: rgba(255, 255, 255, 0.95);
    }

    .section-list li:last-child {
        margin-bottom: 0;
    }

    .section-list li::marker {
        color: rgba(255, 255, 255, 0.3);
    }

    /* Dismiss button - brighter teal with animated gradient border */
    .dismiss-button {
        position: relative;
        width: 100%;
        padding: 0.625rem 1rem;
        border-radius: 0.5rem;
        font-weight: 500;
        font-size: 0.9375rem;
        /* Smooth transitions for all properties including box-shadow */
        transition:
            background-color 0.3s ease,
            transform 0.2s ease,
            box-shadow 1s ease;
        /* Brighter teal background */
        background-color: hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 5%), 0.25);
        color: rgba(255, 255, 255, 0.95);
        border: none;
        cursor: pointer;
        overflow: hidden;
        /* Start with no glow */
        box-shadow: 0 0 0 transparent;
    }

    /* Animated gradient border using mask technique - always visible */
    .dismiss-button::before {
        content: "";
        position: absolute;
        inset: 0;
        border-radius: inherit;
        padding: 1.5px;
        /* Teal-focused gradient: white → light teal → dark teal (doubled) → looping */
        background: linear-gradient(
            90deg,
            rgba(255, 255, 255, 0.5),
            hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 20%), 0.7),
            hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) - 15%), 0.8),
            hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) - 15%), 0.8),
            hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 20%), 0.7)
        );
        background-size: 300% 100%;
        -webkit-mask:
            linear-gradient(#fff 0 0) content-box,
            linear-gradient(#fff 0 0);
        -webkit-mask-composite: xor;
        mask-composite: exclude;
        pointer-events: none;
        /* Slower animation */
        animation: flowGradient 8s linear infinite;
        opacity: 0.75;
        transition: opacity 0.5s ease, padding 0.3s ease;
    }

    @keyframes flowGradient {
        0% { background-position: 0% 0%; }
        100% { background-position: 300% 0%; }
    }

    .dismiss-button:hover {
        background-color: hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 5%), 0.35);
        transform: translateY(-1px);
        /* Subtle teal glow on hover - smooth transition */
        box-shadow: 0 2px 8px hsla(var(--accent-hue), var(--accent-saturation), calc(var(--accent-lightness) + 10%), 0.15);
    }

    /* Stronger border on hover */
    .dismiss-button:hover::before {
        opacity: 1;
        padding: 2px;
    }

    .dismiss-button:active {
        transform: scale(0.98) translateY(0);
    }
</style>
