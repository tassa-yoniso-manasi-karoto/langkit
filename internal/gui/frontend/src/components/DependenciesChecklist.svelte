<script lang="ts">
    import { onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import { fly } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore, settings, systemInfoStore, liteModeStore } from '../lib/stores';
    import { logger } from '../lib/logger';
    import ExternalLink from './ExternalLink.svelte';
    import DockerUnavailableIcon from './icons/DockerUnavailableIcon.svelte';
    import ErrorCard from './ErrorCard.svelte';
    import { OpenExecutableDialog } from '../api/services/media';
    import { DownloadFFmpeg, DownloadMediaInfo } from '../api/services/deps';
    import { SaveSettings } from '../api/services/settings';
    import { wsClient } from '../ws/client';

    export let recheckFFmpeg: () => Promise<void>;
    export let recheckMediaInfo: () => Promise<void>;

    // Reactive variables from stores
    $: dockerStatus = $dockerStatusStore;
    $: internetStatus = $internetStatusStore;
    $: ffmpegStatus = $ffmpegStatusStore;
    $: mediainfoStatus = $mediainfoStatusStore;
    $: dockerReady = dockerStatus.checked;
    $: internetReady = internetStatus.checked;
    $: ffmpegReady = ffmpegStatus.checked;
    $: mediainfoReady = mediainfoStatus.checked;
    
    // Get system info from store
    $: systemInfo = $systemInfoStore;

    // Track lite mode for Qt+Windows compatibility
    $: liteMode = $liteModeStore.enabled;

    // Download states
    let ffmpeg_downloading = false;
    let mediainfo_downloading = false;
  
    let ffmpeg_progress = 0;
    let ffmpeg_description = 'Starting download...';
    let ffmpeg_error: string | null = null;
    let mediainfo_progress = 0;
    let mediainfo_description = 'Starting download...';
    let mediainfo_error: string | null = null;
  
    let ffmpegProgressHandler: ((data: any) => void) | null = null;
    let mediainfoProgressHandler: ((data: any) => void) | null = null;
    
    async function handleDownload(dependency: 'ffmpeg' | 'mediainfo') {
        if (dependency === 'ffmpeg') {
            ffmpeg_error = null;
            ffmpeg_downloading = true;
            ffmpegProgressHandler = (data: any) => {
                ffmpeg_progress = data.progress;
                ffmpeg_description = data.description;
            };
            wsClient.on('download.ffmpeg.progress', ffmpegProgressHandler);
            try {
                await DownloadFFmpeg();
                if (recheckFFmpeg) await recheckFFmpeg();
            } catch (err) {
                logger.error('DependenciesChecklist', 'FFmpeg download failed', { error: err });
                ffmpeg_error = err as string;
            } finally {
                ffmpeg_downloading = false;
                if (ffmpegProgressHandler) {
                    wsClient.off('download.ffmpeg.progress', ffmpegProgressHandler);
                    ffmpegProgressHandler = null;
                }
            }
        } else {
            mediainfo_error = null;
            mediainfo_downloading = true;
            mediainfoProgressHandler = (data: any) => {
                mediainfo_progress = data.progress;
                mediainfo_description = data.description;
            };
            wsClient.on('download.mediainfo.progress', mediainfoProgressHandler);
            try {
                await DownloadMediaInfo();
                if (recheckMediaInfo) await recheckMediaInfo();
            } catch (err) {
                logger.error('DependenciesChecklist', 'MediaInfo download failed', { error: err });
                mediainfo_error = err as string;
            } finally {
                mediainfo_downloading = false;
                if (mediainfoProgressHandler) {
                    wsClient.off('download.mediainfo.progress', mediainfoProgressHandler);
                    mediainfoProgressHandler = null;
                }
            }
        }
    }

    async function handleLocate(dependency: 'ffmpeg' | 'mediainfo') {
        const title = `Select ${dependency} executable`;
        try {
            const path = await OpenExecutableDialog(title);
            if (path) {
                const newSettings = { ...get(settings) };
                if (dependency === 'ffmpeg') {
                    newSettings.ffmpegPath = path;
                } else {
                    newSettings.mediainfoPath = path;
                }
                await SaveSettings(newSettings);
                settings.set(newSettings);

                if (dependency === 'ffmpeg') {
                    if (recheckFFmpeg) await recheckFFmpeg();
                } else {
                    if (recheckMediaInfo) await recheckMediaInfo();
                }
            }
        } catch (err) {
            logger.error('DependenciesChecklist', `Failed to open file dialog for ${dependency}`, { error: err });
        }
    }

    function formatFFmpegVersion(version: string | undefined): string {
        if (!version) {
            return 'detected';
        }
        const match = version.match(/-(\d{8})$/);
        if (match && match[1]) {
            const dateStr = match[1];
            const year = dateStr.substring(0, 4);
            const month = dateStr.substring(4, 6);
            const day = dateStr.substring(6, 8);
            return `Master build (${year}-${month}-${day})`;
        }
        return `v${version}`;
    }

    onDestroy(() => {
        // Clean up any remaining WebSocket handlers
        if (ffmpegProgressHandler) {
            wsClient.off('download.ffmpeg.progress', ffmpegProgressHandler);
        }
        if (mediainfoProgressHandler) {
            wsClient.off('download.mediainfo.progress', mediainfoProgressHandler);
        }
    });
</script>

<div class="space-y-6 px-4 w-full max-w-xl">
    <!-- FFmpeg Status -->
    <div>
        <!-- Conditionally disable backdrop-blur in reduced mode to prevent Qt WebEngine flickering -->
        <div class="flex items-center justify-between p-4 rounded-2xl
                    {liteMode ? 'bg-black/40' : 'backdrop-blur-md'} border border-white/10
                    transition-all duration-300
                    relative overflow-hidden"
             style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1));
                    border-color: rgba(255, 255, 255, var(--style-welcome-border-opacity, 0.1))"
             on:mouseover={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-hover-opacity, 0.15))'}
             on:mouseout={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))'}
             in:fly={{ y: 20, duration: 400, delay: 150, easing: cubicOut }}>
            
            {#if !ffmpegReady}
                <!-- Skeleton loader overlay -->
                <div class="absolute inset-0 animate-skeleton-sweep"></div>
            {/if}
            
            <div class="flex items-center gap-3 relative">
                <div class="w-8 h-8 flex items-center justify-center">
                    {#if ffmpegReady}
                        {#if ffmpegStatus?.available}
                            <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-pale-green">
                                <mask id="ffmpegCheckMask">
                                    <g fill="none" stroke="#fff" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                        <path fill="#fff" fill-opacity="0" stroke-dasharray="64" stroke-dashoffset="64" d="M3 12c0 -4.97 4.03 -9 9 -9c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9Z">
                                            <animate fill="freeze" attributeName="fill-opacity" begin="0.6s" dur="0.5s" values="0;1"/>
                                            <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                        </path>
                                        <path stroke="#000" stroke-dasharray="14" stroke-dashoffset="14" d="M8 12l3 3l5 -5">
                                            <animate fill="freeze" attributeName="stroke-dashoffset" begin="1.1s" dur="0.2s" values="14;0"/>
                                        </path>
                                    </g>
                                </mask>
                                <rect width="24" height="24" fill="currentColor" mask="url(#ffmpegCheckMask)"/>
                            </svg>
                        {:else if ffmpegStatus}
                            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" class="text-red-500">
                                <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                    <path stroke-dasharray="64" stroke-dashoffset="64" d="M12 3c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9c0 -4.97 4.03 -9 9 -9Z">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                    </path>
                                    <path stroke-dasharray="8" stroke-dashoffset="8" d="M12 7v6">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.6s" dur="0.2s" values="8;0"/>
                                        <animate attributeName="stroke-width" begin="1.8s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                    <path stroke-dasharray="2" stroke-dashoffset="2" d="M12 17v0.01">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.8s" dur="0.2s" values="2;0"/>
                                        <animate attributeName="stroke-width" begin="2.1s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                </g>
                            </svg>
                        {:else}
                            <span class="material-icons text-3xl text-gray-400">pending</span>
                        {/if}
                    {:else}
                        <div class="w-8 h-8 rounded-full bg-white/10"></div>
                    {/if}
                </div>
                <div class="text-left">
                    {#if ffmpegReady}
                        <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">FFmpeg</h3>
                        <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                            {#if ffmpegStatus?.available}
                                {formatFFmpegVersion(ffmpegStatus.version)}
                            {:else if ffmpegStatus?.error}
                                {ffmpegStatus.error}
                            {:else}
                                Checking availability...
                            {/if}
                        </p>
                    {:else}
                        <div class="h-5 rounded w-32 mb-1" style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))"></div>
                        <div class="h-3.5 rounded w-48" style="background-color: rgba(255, 255, 255, 0.05)"></div>
                    {/if}
                </div>
            </div>
            
            <div class="flex-shrink-0 ml-auto w-2/5">
                {#if ffmpegReady && ffmpegStatus && !ffmpegStatus.available}
                    {#if ffmpeg_downloading}
                        <div class="flex flex-col items-center">
                            <div class="text-xs text-gray-400 text-center mb-1">{ffmpeg_description}</div>
                            <div class="w-full bg-gray-700 rounded-full h-2.5">
                                <div class="bg-primary h-2.5 rounded-full" style="width: {ffmpeg_progress}%"></div>
                            </div>
                        </div>
                    {:else if systemInfo.os === 'linux'}
                        <p class="text-xs text-gray-400 text-center">Please install using your package manager.</p>
                    {:else}
                        <div class="flex flex-col items-end gap-2">
                            {#if ffmpeg_error}
                                <p class="text-xs text-red-400 text-right">Error: {ffmpeg_error}</p>
                            {/if}
                            <button on:click={() => handleDownload('ffmpeg')} disabled={ffmpeg_downloading} class="px-3 py-1.5 text-xs font-medium text-white bg-primary rounded-md hover:bg-primary/80 disabled:bg-gray-500 w-full text-center">Download Automatically</button>
                            <button on:click={() => handleLocate('ffmpeg')} class="px-3 py-1.5 text-xs font-medium text-white bg-gray-600 rounded-md hover:bg-gray-500 w-full text-center">Locate Manually</button>
                        </div>
                    {/if}
                {/if}
            </div>
        </div>
        
        <ErrorCard 
            show={ffmpegReady && ffmpegStatus && !ffmpegStatus.available}
            message="<strong>FFmpeg is required</strong> for all media processing operations. Without it, Langkit cannot function." 
        />
    </div>
    
    <!-- MediaInfo Status -->
    <div>
        <!-- Conditionally disable backdrop-blur in reduced mode to prevent Qt WebEngine flickering -->
        <div class="flex items-center justify-between p-4 rounded-2xl
                    {liteMode ? 'bg-black/40' : 'backdrop-blur-md'} border border-white/10
                    transition-all duration-300
                    relative overflow-hidden"
             style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1));
                    border-color: rgba(255, 255, 255, var(--style-welcome-border-opacity, 0.1))"
             on:mouseover={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-hover-opacity, 0.15))'}
             on:mouseout={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))'}
             in:fly={{ y: 20, duration: 400, delay: 200, easing: cubicOut }}>
            
            {#if !mediainfoReady}
                <!-- Skeleton loader overlay -->
                <div class="absolute inset-0 animate-skeleton-sweep"></div>
            {/if}
            
            <div class="flex items-center gap-3 relative">
                <div class="w-8 h-8 flex items-center justify-center">
                    {#if mediainfoReady}
                        {#if mediainfoStatus?.available}
                            <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-pale-green">
                                <mask id="mediainfoCheckMask">
                                    <g fill="none" stroke="#fff" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                        <path fill="#fff" fill-opacity="0" stroke-dasharray="64" stroke-dashoffset="64" d="M3 12c0 -4.97 4.03 -9 9 -9c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9Z">
                                            <animate fill="freeze" attributeName="fill-opacity" begin="0.6s" dur="0.5s" values="0;1"/>
                                            <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                        </path>
                                        <path stroke="#000" stroke-dasharray="14" stroke-dashoffset="14" d="M8 12l3 3l5 -5">
                                            <animate fill="freeze" attributeName="stroke-dashoffset" begin="1.1s" dur="0.2s" values="14;0"/>
                                        </path>
                                    </g>
                                </mask>
                                <rect width="24" height="24" fill="currentColor" mask="url(#mediainfoCheckMask)"/>
                            </svg>
                        {:else if mediainfoStatus}
                            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" class="text-red-500">
                                <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                    <path stroke-dasharray="64" stroke-dashoffset="64" d="M12 3c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9c0 -4.97 4.03 -9 9 -9Z">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                    </path>
                                    <path stroke-dasharray="8" stroke-dashoffset="8" d="M12 7v6">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.6s" dur="0.2s" values="8;0"/>
                                        <animate attributeName="stroke-width" begin="1.8s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                    <path stroke-dasharray="2" stroke-dashoffset="2" d="M12 17v0.01">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.8s" dur="0.2s" values="2;0"/>
                                        <animate attributeName="stroke-width" begin="2.1s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                </g>
                            </svg>
                        {:else}
                            <span class="material-icons text-3xl text-gray-400">pending</span>
                        {/if}
                    {:else}
                        <div class="w-8 h-8 rounded-full bg-white/10"></div>
                    {/if}
                </div>
                <div class="text-left">
                    {#if mediainfoReady}
                        <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">MediaInfo</h3>
                        <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                            {#if mediainfoStatus?.available}
                                v{mediainfoStatus.version || 'detected'}
                            {:else if mediainfoStatus?.error}
                                {mediainfoStatus.error}
                            {:else}
                                Checking availability...
                            {/if}
                        </p>
                    {:else}
                        <div class="h-5 rounded w-32 mb-1" style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))"></div>
                        <div class="h-3.5 rounded w-48" style="background-color: rgba(255, 255, 255, 0.05)"></div>
                    {/if}
                </div>
            </div>
            
            <div class="flex-shrink-0 ml-auto w-2/5">
                {#if mediainfoReady && mediainfoStatus && !mediainfoStatus.available}
                    {#if mediainfo_downloading}
                        <div class="flex flex-col items-center">
                            <div class="text-xs text-gray-400 text-center mb-1">{mediainfo_description}</div>
                            <div class="w-full bg-gray-700 rounded-full h-2.5">
                                <div class="bg-primary h-2.5 rounded-full" style="width: {mediainfo_progress}%"></div>
                            </div>
                        </div>
                    {:else if systemInfo.os === 'linux'}
                        <p class="text-xs text-gray-400 text-center">Please install using your package manager.</p>
                    {:else}
                        <div class="flex flex-col items-end gap-2">
                            {#if mediainfo_error}
                                <p class="text-xs text-red-400 text-right">Error: {mediainfo_error}</p>
                            {/if}
                            <button on:click={() => handleDownload('mediainfo')} disabled={mediainfo_downloading} class="px-3 py-1.5 text-xs font-medium text-white bg-primary rounded-md hover:bg-primary/80 disabled:bg-gray-500 w-full text-center">Download Automatically</button>
                            <button on:click={() => handleLocate('mediainfo')} class="px-3 py-1.5 text-xs font-medium text-white bg-gray-600 rounded-md hover:bg-gray-500 w-full text-center">Locate Manually</button>
                        </div>
                    {/if}
                {/if}
            </div>
        </div>
        
        <ErrorCard 
            show={mediainfoReady && mediainfoStatus && !mediainfoStatus.available}
            message="<strong>MediaInfo is required</strong> for media file analysis. Without it, Langkit cannot process media files." 
        />
    </div>
    
    <!-- Docker Status -->
    <div>
        <!-- Conditionally disable backdrop-blur in reduced mode to prevent Qt WebEngine flickering -->
        <div class="flex items-center justify-between p-4 rounded-2xl
                    {liteMode ? 'bg-black/40' : 'backdrop-blur-md'} border border-white/10
                    transition-all duration-300
                    relative overflow-hidden"
             style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1));
                    border-color: rgba(255, 255, 255, var(--style-welcome-border-opacity, 0.1))"
             on:mouseover={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-hover-opacity, 0.15))'}
             on:mouseout={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))'}
             in:fly={{ y: 20, duration: 400, delay: 50, easing: cubicOut }}>
            
            {#if !dockerReady}
                <!-- Skeleton loader overlay -->
                <div class="absolute inset-0 animate-skeleton-sweep"></div>
            {/if}
            
            <div class="flex items-center gap-3 relative">
                <div class="w-8 h-8 flex items-center justify-center">
                    {#if dockerReady}
                        {#if dockerStatus?.available}
                            <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-pale-green">
                                <mask id="dockerCheckMask">
                                    <g fill="none" stroke="#fff" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                        <path fill="#fff" fill-opacity="0" stroke-dasharray="64" stroke-dashoffset="64" d="M3 12c0 -4.97 4.03 -9 9 -9c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9Z">
                                            <animate fill="freeze" attributeName="fill-opacity" begin="0.6s" dur="0.5s" values="0;1"/>
                                            <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                        </path>
                                        <path stroke="#000" stroke-dasharray="14" stroke-dashoffset="14" d="M8 12l3 3l5 -5">
                                            <animate fill="freeze" attributeName="stroke-dashoffset" begin="1.1s" dur="0.2s" values="14;0"/>
                                        </path>
                                    </g>
                                </mask>
                                <rect width="24" height="24" fill="currentColor" mask="url(#dockerCheckMask)"/>
                            </svg>
                        {:else if dockerStatus}
                            <DockerUnavailableIcon size="48" className="text-blue-400" />
                        {:else}
                            <span class="material-icons text-3xl text-gray-400">pending</span>
                        {/if}
                    {:else}
                        <div class="w-8 h-8 rounded-full bg-white/10"></div>
                    {/if}
                </div>
                <div class="text-left">
                    {#if dockerReady}
                        <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">{dockerStatus?.engine || 'Docker Desktop'}</h3>
                        <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                            {#if dockerStatus?.available}
                                v{dockerStatus.version || 'detected'}
                            {:else if dockerStatus?.error}
                                {dockerStatus.error}
                            {:else}
                                Checking availability...
                            {/if}
                        </p>
                    {:else}
                        <div class="h-5 rounded w-32 mb-1" style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))"></div>
                        <div class="h-3.5 rounded w-48" style="background-color: rgba(255, 255, 255, 0.05)"></div>
                    {/if}
                </div>
            </div>
            
            {#if dockerReady && dockerStatus && !dockerStatus.available}
                {#if systemInfo.os === 'linux'}
                    <p class="text-xs text-gray-400">Please install Docker using your distribution's package manager.</p>
                {:else}
                    <ExternalLink
                        href="https://docs.docker.com/get-docker/"
                        className="text-primary hover:text-primary/80"
                        title="">
                        <span class="material-icons text-sm text-primary hover:text-primary/80">open_in_new</span>
                    </ExternalLink>
                {/if}
            {/if}
        </div>
        
        <ErrorCard 
            show={dockerReady && dockerStatus && !dockerStatus.available}
            message="Linguistic processing for <strong>Japanese & Indic languages</strong> will not be available so subtitle-related features for <strong>these languages will be out of service</strong>." 
        />
    </div>
    
    <!-- Internet Status -->
    <div>
        <!-- Conditionally disable backdrop-blur in reduced mode to prevent Qt WebEngine flickering -->
        <div class="flex items-center justify-between p-4 rounded-2xl
                    {liteMode ? 'bg-black/40' : 'backdrop-blur-md'} border border-white/10
                    transition-all duration-300
                    relative overflow-hidden"
             style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1));
                    border-color: rgba(255, 255, 255, var(--style-welcome-border-opacity, 0.1))"
             on:mouseover={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-hover-opacity, 0.15))'}
             on:mouseout={(e) => e.currentTarget.style.backgroundColor = 'rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))'}
             in:fly={{ y: 20, duration: 400, delay: 100, easing: cubicOut }}>
            
            {#if !internetReady}
                <!-- Skeleton loader overlay -->
                <div class="absolute inset-0 animate-skeleton-sweep"></div>
            {/if}
            
            <div class="flex items-center gap-3 relative">
                <div class="w-8 h-8 flex items-center justify-center">
                    {#if internetReady}
                        {#if internetStatus?.online}
                            <svg width="32" height="32" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" class="text-pale-green">
                                <style>
                                    .wifi_anim1{animation:wifi_fade 3s linear infinite}
                                    .wifi_anim2{animation:wifi_fade2 3s linear infinite}
                                    .wifi_anim3{animation:wifi_fade3 3s linear infinite}
                                    @keyframes wifi_fade{0%,32.26%,100%{opacity:0}48.39%,93.55%{opacity:1}}
                                    @keyframes wifi_fade2{0%,48.39%,100%{opacity:0}64.52%,93.55%{opacity:1}}
                                    @keyframes wifi_fade3{0%,64.52%,100%{opacity:0}80.65%,93.55%{opacity:1}}
                                </style>
                                <path class="wifi_anim1" fill="currentColor" d="M12,21L15.6,16.2C14.6,15.45 13.35,15 12,15C10.65,15 9.4,15.45 8.4,16.2L12,21" opacity="0"/>
                                <path class="wifi_anim1 wifi_anim2" fill="currentColor" d="M12,9C9.3,9 6.81,9.89 4.8,11.4L6.6,13.8C8.1,12.67 9.97,12 12,12C14.03,12 15.9,12.67 17.4,13.8L19.2,11.4C17.19,9.89 14.7,9 12,9Z" opacity="0"/>
                                <path class="wifi_anim1 wifi_anim3" fill="currentColor" d="M12,3C7.95,3 4.21,4.34 1.2,6.6L3,9C5.5,7.12 8.62,6 12,6C15.38,6 18.5,7.12 21,9L22.8,6.6C19.79,4.34 16.05,3 12,3" opacity="0"/>
                            </svg>
                        {:else if internetStatus}
                            <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" class="text-red-500">
                                <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                    <path stroke-dasharray="64" stroke-dashoffset="64" d="M12 3c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9c0 -4.97 4.03 -9 9 -9Z">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.6s" values="64;0"/>
                                    </path>
                                    <path stroke-dasharray="8" stroke-dashoffset="8" d="M12 7v6">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.6s" dur="0.2s" values="8;0"/>
                                        <animate attributeName="stroke-width" begin="1.8s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                    <path stroke-dasharray="2" stroke-dashoffset="2" d="M12 17v0.01">
                                        <animate fill="freeze" attributeName="stroke-dashoffset" begin="0.8s" dur="0.2s" values="2;0"/>
                                        <animate attributeName="stroke-width" begin="2.1s" dur="3s" keyTimes="0;0.1;0.2;0.3;1" repeatCount="indefinite" values="2;3;3;2;2"/>
                                    </path>
                                </g>
                            </svg>
                        {:else}
                            <span class="material-icons text-3xl text-gray-400">pending</span>
                        {/if}
                    {:else}
                        <div class="w-8 h-8 rounded-full bg-white/10"></div>
                    {/if}
                </div>
                <div class="text-left">
                    {#if internetReady}
                        <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">Internet Connection</h3>
                        <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                            {#if internetStatus?.online}
                                Connected ({internetStatus.latency}ms latency)
                            {:else if internetStatus?.error}
                                {internetStatus.error}
                            {:else}
                                Checking connectivity...
                            {/if}
                        </p>
                    {:else}
                        <div class="h-5 rounded w-32 mb-1" style="background-color: rgba(255, 255, 255, var(--style-welcome-card-bg-opacity, 0.1))"></div>
                        <div class="h-3.5 rounded w-48" style="background-color: rgba(255, 255, 255, 0.05)"></div>
                    {/if}
                </div>
            </div>
        </div>
        
        <ErrorCard 
            show={internetReady && internetStatus && !internetStatus.online}
            message="An internet connection is required for AI-powered features. Dubtitles, voice enhancing and subtitle processing for certain languages will not be available offline." 
        />
    </div>
</div>

<style>
    /* Subtle skeleton sweep animation */
    @keyframes skeleton-sweep {
        0% { 
            transform: translateX(-100%);
        }
        100% { 
            transform: translateX(100%);
        }
    }
    
    .animate-skeleton-sweep {
        animation: skeleton-sweep 1.2s ease-in-out infinite;
        background: linear-gradient(
            90deg, 
            transparent 0%, 
            rgba(255, 255, 255, 0.03) 35%, 
            rgba(255, 255, 255, 0.08) 50%, 
            rgba(255, 255, 255, 0.03) 65%, 
            transparent 100%
        );
    }
</style>