<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fade, scale, fly } from 'svelte/transition';
    import { cubicOut, backOut, elasticOut } from 'svelte/easing';
import { get } from 'svelte/store';
    import { statisticsStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore, settings } from '../lib/stores';
    import { logger } from '../lib/logger';
    import ExternalLink from './ExternalLink.svelte';
    import DockerUnavailableIcon from './icons/DockerUnavailableIcon.svelte';
    import DownloadProgress from './DownloadProgress.svelte';
    import { OpenExecutableDialog, DownloadFFmpeg, CheckFFmpegAvailability, CheckMediaInfoAvailability } from '../../wailsjs/go/gui/App';
    import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
    
    // Custom slide projector transition
    function slideProjector(node, {
        delay = 0,
        duration = 800,
        easing = cubicOut,
        yStart = 100,
        yEnd = 0,
        scaleStart = 0.8,
        scaleEnd = 1,
        opacityStart = 0,
        opacityEnd = 1,
        blurStart = 0,
        blurEnd = 0
    }) {
        const o = +getComputedStyle(node).opacity;
        return {
            delay,
            duration,
            easing,
            css: (t) => {
                const y = yStart + (yEnd - yStart) * t;
                const scale = scaleStart + (scaleEnd - scaleStart) * t;
                const opacity = opacityStart + (opacityEnd - opacityStart) * t;
                const blur = blurStart + (blurEnd - blurStart) * t;
                return `
                    transform: translateY(${y}px) scale(${scale});
                    opacity: ${opacity * o};
                    filter: blur(${blur}px);
                `;
            }
        };
    }
    
    // Custom slide projector OUT transition (properly handles reverse)
    function slideProjectorOut(node, {
        delay = 0,
        duration = 800,
        easing = cubicOut
    }) {
        return {
            delay,
            duration,
            easing,
            css: (t) => {
                // For out transition, we want to go from current position UP and fade/shrink
                // t goes from 1 to 0, so we use (1-t) to get proper direction
                const progress = 1 - t;
                const y = -150 * progress; // Moves up as progress increases
                const scale = 1 - (0.35 * progress); // Shrinks from 1 to 0.65
                const opacity = 1 - progress; // Fades from 1 to 0
                const blur = 6 * progress; // Increases blur
                return `
                    transform: translateY(${y}px) scale(${scale});
                    opacity: ${opacity};
                    filter: blur(${blur}px);
                `;
            }
        };
    }
    
    export let onClose: () => void = () => {};
    
    // State variables
    let showWelcome = true;
    let showApiKeys = false;
    let showLanguages = false;
    
    // Reactive variables from stores
    $: dockerStatus = $dockerStatusStore;
    $: internetStatus = $internetStatusStore;
    $: ffmpegStatus = $ffmpegStatusStore;
    $: mediainfoStatus = $mediainfoStatusStore;
    $: dockerReady = dockerStatus.checked;
    $: internetReady = internetStatus.checked;
    $: ffmpegReady = ffmpegStatus.checked;
    $: mediainfoReady = mediainfoStatus.checked;
    
    // Animation states
    let titleVisible = false;
    let contentVisible = false;
    let actionsVisible = false;
    
    // Step management
    let currentStep = 0; // 0: requirements, 1: api keys, 2: get started
    
    // Button ripple effect
    let getStartedButton: HTMLButtonElement | null = null;
    
    function handleMouseEnter(event: MouseEvent) {
        if (!getStartedButton) return;
        
        // Get exact coordinates relative to the button
        const rect = getStartedButton.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        
        // Create and style a new element for the fill effect
        const fill = document.createElement('div');
        fill.style.position = 'absolute';
        fill.style.left = x + 'px';
        fill.style.top = y + 'px';
        fill.style.width = '0';
        fill.style.height = '0';
        fill.style.borderRadius = '50%';
        fill.style.backgroundColor = 'hsla(320, 90%, 60%, 0.3)';
        fill.style.transform = 'translate(-50%, -50%)';
        fill.style.transition = 'width 0.5s ease-out, height 0.5s ease-out';
        fill.style.zIndex = '-1';
        
        // Append to button
        getStartedButton.style.position = 'relative';
        getStartedButton.style.overflow = 'hidden';
        getStartedButton.appendChild(fill);
        
        // Force reflow
        fill.offsetWidth;
        
        // Calculate max dimension needed to fill button
        const maxDim = Math.max(
            rect.width * 2,
            rect.height * 2,
            Math.sqrt(Math.pow(x, 2) + Math.pow(y, 2)) * 2,
            Math.sqrt(Math.pow(rect.width - x, 2) + Math.pow(y, 2)) * 2,
            Math.sqrt(Math.pow(x, 2) + Math.pow(rect.height - y, 2)) * 2,
            Math.sqrt(Math.pow(rect.width - x, 2) + Math.pow(rect.height - y, 2)) * 2
        );
        
        // Expand the fill
        fill.style.width = maxDim + 'px';
        fill.style.height = maxDim + 'px';
    }
    
    function handleMouseLeave() {
        // Remove all fill elements when mouse leaves
        if (getStartedButton) {
            const fills = getStartedButton.querySelectorAll('div');
            fills.forEach((fill: Element) => getStartedButton?.removeChild(fill));
        }
    }
    
    // Keyboard handling
    function handleKeydown(e: KeyboardEvent) {
        if (e.key === 'Escape') {
            onClose();
        } else if (e.key === 'ArrowLeft' && showApiKeys) {
            goToPage(0);
        } else if (e.key === 'ArrowRight' && showWelcome) {
            handleNext();
        }
    }
    
    
    // Check statuses on mount
    onMount(async () => {
        logger.info('WelcomePopup', 'Welcome popup mounted');
        
        // Start reveal animation sequence
        setTimeout(() => titleVisible = true, 100);
        setTimeout(() => contentVisible = true, 300);
        setTimeout(() => actionsVisible = true, 500);
        
        // Add keyboard listener
        window.addEventListener('keydown', handleKeydown);
    });
    
    onDestroy(() => {
        window.removeEventListener('keydown', handleKeydown);
    });
    
    function handleNext() {
        if (showWelcome) {
            logger.info('WelcomePopup', 'Moving to API keys explanation');
            showWelcome = false;
            setTimeout(() => {
                showApiKeys = true;
                currentStep = 1;
            }, 50);
        } else if (showApiKeys) {
            logger.info('WelcomePopup', 'Moving to languages explanation');
            showApiKeys = false;
            setTimeout(() => {
                showLanguages = true;
                currentStep = 2;
            }, 50);
        } else if (showLanguages) {
            logger.info('WelcomePopup', 'User clicked Get Started');
            onClose();
        }
    }
    
    function goToPage(page: number) {
        if (page === 0 && !showWelcome) {
            logger.info('WelcomePopup', 'Switching to Welcome page');
            showApiKeys = false;
            showLanguages = false;
            setTimeout(() => {
                showWelcome = true;
                currentStep = 0;
            }, 50);
        } else if (page === 1 && !showApiKeys) {
            logger.info('WelcomePopup', 'Switching to API Keys page');
            showWelcome = false;
            showLanguages = false;
            setTimeout(() => {
                showApiKeys = true;
                currentStep = 1;
            }, 50);
        } else if (page === 2 && !showLanguages) {
            logger.info('WelcomePopup', 'Switching to Languages page');
            showWelcome = false;
            showApiKeys = false;
            setTimeout(() => {
                showLanguages = true;
                currentStep = 2;
            }, 50);
        }
    }
    
    let ffmpeg_downloading = false;

    async function handleDownload(dependency: 'ffmpeg' | 'mediainfo') {
        if (dependency === 'ffmpeg') {
            ffmpeg_downloading = true;
            try {
                await DownloadFFmpeg();
            } catch (err) {
                logger.error('WelcomePopup', 'FFmpeg download failed', { error: err });
            } finally {
                ffmpeg_downloading = false;
            }
        } else {
            BrowserOpenURL('https://mediaarea.net/en/MediaInfo/Download');
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
                await window.go.gui.App.SaveSettings(newSettings);
                settings.set(newSettings);

                if (dependency === 'ffmpeg') {
                    checkFFmpegAvailability();
                } else {
                    checkMediaInfoAvailability();
                }
            }
        } catch (err) {
            logger.error('WelcomePopup', `Failed to open file dialog for ${dependency}`, { error: err });
        }
    }

    $: buttonText = showWelcome ? 'Continue' : showApiKeys ? 'Continue' : 'Get Started';
</script>

<div class="fixed inset-0 z-[9999] flex items-center justify-center p-4 backdrop-blur-sm"
     in:fade={{ duration: 300 }}
     out:fade={{ duration: 200 }}>
    
    <!-- Background overlay -->
    <div class="absolute inset-0" style="background-color: rgba(0, 0, 0, var(--style-welcome-overlay-opacity, 0.4))" on:click={onClose}></div>
    
    <!-- Popup container with fixed size -->
    <div class="relative max-w-2xl w-full"
         in:scale={{ duration: 400, easing: backOut, start: 0.95 }}
         out:scale={{ duration: 200, easing: cubicOut, start: 0.95 }}>
        
        <!-- Panel with more solid appearance and depth -->
        <div class="relative overflow-hidden rounded-3xl
                    shadow-[0_20px_50px_-12px_rgba(0,0,0,0.8)]
                    panel-glow"
             style="background-color: rgba(0, 0, 0, var(--style-welcome-panel-bg-opacity, 0.3));
                    backdrop-filter: blur(var(--style-welcome-panel-blur, 24px));
                    -webkit-backdrop-filter: blur(var(--style-welcome-panel-blur, 24px));
                    border: 1px solid rgba(255, 255, 255, var(--style-welcome-border-opacity, 0.1))">
            
            <!-- Subtle gradient accent -->
            <div class="absolute -top-32 -right-32 w-64 h-64 rounded-full bg-primary/10 blur-3xl"></div>
            
            <!-- Content -->
            <div class="relative pt-8 md:pt-12 px-8 md:px-12 pb-6 md:pb-10">
                <!-- Content container with 3D slide effect -->
                <div class="min-h-[520px] max-h-[60vh] relative slide-container overflow-x-hidden overflow-y-auto mask-fade">
                    <!-- Step 0: Welcome page -->
                    {#if showWelcome}
                        <div class="absolute top-0 left-0 right-0 flex flex-col items-center pb-5"
                             in:slideProjector={{ yStart: 100, yEnd: 0, scaleStart: 0.8, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, duration: 800 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            <!-- Welcome header -->
                            <div class="text-center mb-8 pt-4">
                                <h1 class="text-4xl md:text-5xl font-[Outfit] font-bold mb-3 flex items-center justify-center gap-2"
                                    style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    Welcome to Langkit
                                    <span class="waving-hand text-4xl md:text-5xl">👋</span>
                                </h1>
                                <p class="text-lg min-h-[28px]"
                                   style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                    Let's check your system requirements
                                </p>
                            </div>
                            
                            <!-- System requirements cards -->
                            <div class="space-y-6 px-4 w-full max-w-xl">
                        <!-- Docker Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        backdrop-blur-md border border-white/10
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
                                                <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-primary">
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
                                    <div>
                                        {#if dockerReady}
                                            <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">{dockerStatus?.engine || 'Docker Desktop'}</h3>
                                            <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                                                {#if dockerStatus?.available}
                                                    {dockerStatus.engine || 'Docker'} v{dockerStatus.version || 'detected'}
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
                                    <ExternalLink
                                        href=https://docs.docker.com/get-docker/
                                        className="text-primary hover:text-primary/80"
                                        title="">
                                        <span class="material-icons text-sm text-primary hover:text-primary/80">open_in_new</span>
                                    </ExternalLink>
                                {/if}
                            </div>
                            
                            {#if dockerReady && dockerStatus && !dockerStatus.available}
                                <div class="px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20"
                                     in:fade={{ duration: 300 }}>
                                    <p class="text-sm text-red-200/80">
                                        Linguistic processing for <strong>Japanese & Indic languages</strong> will not be available so subtitle-related features for <strong>these languages will be out of service</strong>.
                                    </p>
                                </div>
                            {/if}
                        </div>
                        
                        <!-- Internet Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        backdrop-blur-md border border-white/10
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
                                    <div>
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
                            
                            {#if internetReady && internetStatus && !internetStatus.online}
                                <div class="px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20"
                                     in:fade={{ duration: 300 }}>
                                    <p class="text-sm text-red-200/80">
                                        An internet connection is required for AI-powered features.
                                        Dubtitles, voice enhancing and subtitle processing for certain languages will not be available offline.
                                    </p>
                                </div>
                            {/if}
                        </div>
                        
                        <!-- FFmpeg Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        backdrop-blur-md border border-white/10
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
                                                <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-primary">
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
                                    <div>
                                        {#if ffmpegReady}
                                            <h3 class="font-medium" style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">FFmpeg</h3>
                                            <p class="text-sm" style="color: rgba(255, 255, 255, var(--style-welcome-text-tertiary-opacity, 0.6))">
                                                {#if ffmpegStatus?.available}
                                                    v{ffmpegStatus.version || 'detected'}
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
                                
                            <div class="flex items-center gap-4">
                                {#if ffmpegReady && ffmpegStatus && !ffmpegStatus.available}
                                    <div class="flex gap-2">
                                        <button on:click={() => handleDownload('ffmpeg')} class="px-3 py-1.5 text-xs font-medium text-white bg-primary rounded-md hover:bg-primary/80">Download Automatically</button>
                                        <button on:click={() => handleLocate('ffmpeg')} class="px-3 py-1.5 text-xs font-medium text-white bg-gray-600 rounded-md hover:bg-gray-500">Locate Manually</button>
                                    </div>
                                {/if}
                                </div>
                                {#if ffmpeg_downloading}
                                    <DownloadProgress taskId="ffmpeg-download" />
                                {/if}
                            </div>
                            
                            {#if ffmpegReady && ffmpegStatus && !ffmpegStatus.available}
                                <div class="px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20"
                                     in:fade={{ duration: 300 }}>
                                    <p class="text-sm text-red-200/80">
                                        <strong>FFmpeg is required</strong> for all media processing operations. Without it, Langkit cannot function.
                                    </p>
                                </div>
                            {/if}
                        </div>
                        
                        <!-- MediaInfo Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        backdrop-blur-md border border-white/10
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
                                                <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-primary">
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
                                    <div>
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
                                
                            <div class="flex items-center gap-4">
                                {#if mediainfoReady && mediainfoStatus && !mediainfoStatus.available}
                                    <div class="flex gap-2">
                                        <button on:click={() => handleDownload('mediainfo')} class="px-3 py-1.5 text-xs font-medium text-white bg-primary rounded-md hover:bg-primary/80">Download from Website</button>
                                        <button on:click={() => handleLocate('mediainfo')} class="px-3 py-1.5 text-xs font-medium text-white bg-gray-600 rounded-md hover:bg-gray-500">Locate Manually</button>
                                    </div>
                                {/if}
                            </div>
                        </div>
                        
                        {#if mediainfoReady && mediainfoStatus && !mediainfoStatus.available}
                            <div class="px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20"
                                 in:fade={{ duration: 300 }}>
                                <p class="text-sm text-red-200/80">
                                    <strong>MediaInfo is required</strong> for media file analysis. Without it, Langkit cannot process media files.
                                </p>
                            </div>
                        {/if}
                            </div>
                            </div>
                        </div>
                    {/if}
                    
                    <!-- Step 1: API Keys page -->
                    {#if showApiKeys}
                        <div class="absolute top-0 left-0 right-0 flex flex-col items-center justify-center pt-5 pb-5"
                             in:slideProjector={{ yStart: 150, yEnd: 0, scaleStart: 0.65, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, blurStart: 6, blurEnd: 0, duration: 800, delay: 0 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            
                            <div class="text-center max-w-lg mx-auto px-4">
                                <!-- API Keys header grouped with content -->
                                <h2 class="text-3xl font-semibold mb-6 flex items-center justify-center gap-2"
                                    style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 32 32" class="text-primary" stroke-width="1" stroke="currentColor">
                                        <path fill="currentColor" d="M28 26c-.178 0-.347.03-.511.074l-1.056-1.055c.352-.595.567-1.28.567-2.019s-.215-1.424-.567-2.019l1.055-1.055c.165.043.334.074.512.074a2 2 0 1 0-2-2c0 .178.03.347.074.512l-1.055 1.055C24.424 19.215 23.739 19 23 19s-1.424.215-2.019.567l-1.055-1.055c.043-.165.074-.334.074-.512a2 2 0 1 0-2 2c.178 0 .347-.03.512-.074l1.055 1.055C19.215 21.576 19 22.261 19 23s.215 1.424.567 2.019l-1.055 1.055A2 2 0 0 0 18 26a2 2 0 1 0 2 2c0-.178-.03-.347-.074-.512l1.055-1.055c.595.352 1.28.567 2.019.567s1.424-.215 2.019-.567l1.055 1.055A2 2 0 0 0 26 28a2 2 0 1 0 2-2m-7-3c0-1.102.897-2 2-2s2 .898 2 2s-.897 2-2 2s-2-.897-2-2"/>
                                        <circle cx="22" cy="10" r="2" fill="currentColor"/>
                                        <path fill="currentColor" d="M21 2c-4.963 0-9 4.037-9 9c0 .779.099 1.547.294 2.291L2 23.586V30h6.414l7-7l-2.707-2.707l-1.414 1.414L12.586 23l-1.59 1.59l-1.287-1.295l-1.418 1.41l1.29 1.299L7.587 28H4v-3.586l9.712-9.712l.856-.867l-.199-.585A7.008 7.008 0 0 1 21 4c3.86 0 7.001 3.14 7.001 7h2c0-4.963-4.037-9-9-9Z"/>
                                    </svg>
                                    Understanding API Keys
                                </h2>
                                
                                <div class="space-y-4">
                                    <p class="text-base leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        API keys are <strong>secure, confidential codes</strong> that allow Langkit to access cloud-based AI services.
                                    </p>
                                    <p class="text-base leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        <strong>
                                            An API key is like your private electricity meter number that tracks how much power you use so the company can bill you accurately.<br>
                                            Don't share it with anyone!
                                        </strong>
                                    </p>
                                    
                                    <p class="text-base leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                        They enable powerful features like speech-to-text, subtitle summarization and voice enhancement at a low cost without requiring expensive local hardware.
                                    </p>
                                </div>
                            </div>
                        </div>
                    {/if}

                    <!-- Step 2: Languages page -->
                    {#if showLanguages}
                        <div class="absolute top-0 left-0 right-0 flex flex-col items-center justify-center pt-5 pb-5"
                                in:slideProjector={{ yStart: 150, yEnd: 0, scaleStart: 0.65, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, blurStart: 6, blurEnd: 0, duration: 800, delay: 0 }}
                                out:slideProjectorOut={{ duration: 800 }}>
                            <div class="text-center max-w-lg mx-auto px-4">
                                <h2 class="text-3xl font-semibold mb-6 flex items-center justify-center gap-2"
                                    style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    <span class="material-icons text-primary">translate</span>
                                    A Note on Specific Languages
                                </h2>
                                <div class="space-y-4">
                                    <p class="text-base leading-relaxed"
                                        style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        For some languages with complex writing systems (like Japanese and Indic scripts), Langkit uses powerful external tools.
                                    </p>
                                    <p class="text-base leading-relaxed"
                                        style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        These tools run inside a system called <strong>Docker Desktop</strong>.
                                    </p>
                                    <p class="text-base leading-relaxed"
                                        style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                        Windows Home users that Docker requires a one-time setup of the <strong>Windows Subsystem for Linux (WSL)</strong>.
                                    </p>
                                    <div class="pt-4">
                                        <button on:click={() => BrowserOpenURL('https://docs.microsoft.com/en-us/windows/wsl/install')} class="px-6 py-2.5 rounded-lg font-medium transition-colors duration-300 relative overflow-hidden hover:shadow-lg hover:shadow-primary/30 focus:outline-none focus:ring-2 focus:ring-primary/50 active:scale-[0.97] will-change-transform border" style="background-color: rgba(159, 110, 247, var(--style-welcome-button-bg-opacity, 0.7)); border-color: rgba(159, 110, 247, var(--style-welcome-button-border-opacity, 0.5)); color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                            View Official Installation Guides
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    {/if}
                </div>
                
                <!-- Actions -->
                {#if actionsVisible}
                    <div class="flex flex-col items-center gap-4 mt-4 pb-2"
                         in:fly={{ y: 20, duration: 600, easing: cubicOut }}>
                        
                        <!-- Progress dots -->
                        <div class="flex gap-2">
                            <div 
                                class="w-2 h-2 rounded-full transition-all duration-300 {currentStep === 0 ? 'bg-primary' : ''}"
                                style="{currentStep !== 0 ? 'background-color: rgba(255, 255, 255, var(--style-welcome-progress-dot-opacity, 0.3))' : ''}"
                                on:click={() => goToPage(0)}>
                            </div>
                            <div 
                                class="w-2 h-2 rounded-full transition-all duration-300 {currentStep === 1 ? 'bg-primary' : ''}"
                                style="{currentStep !== 1 ? 'background-color: rgba(255, 255, 255, var(--style-welcome-progress-dot-opacity, 0.3))' : ''}"
                                on:click={() => goToPage(1)}>
                            </div>
                            <div 
                                class="w-2 h-2 rounded-full transition-all duration-300 {currentStep === 2 ? 'bg-primary' : ''}"
                                style="{currentStep !== 2 ? 'background-color: rgba(255, 255, 255, var(--style-welcome-progress-dot-opacity, 0.3))' : ''}"
                                on:click={() => goToPage(2)}>
                            </div>
                        </div>
                        <!--
			  `will-change-transform` fixes a color shift during `active:scale`.
			  The button has a semi-transparent bg (`bg-primary/70`) and a
			  JS-injected semi-transparent ripple (with `z-index: -1`).
			  Scaling these stacked transparent layers without this hint can
			  cause inconsistent color blending by the browser.
			  `will-change` allows the browser to optimize rendering, often by
			  promoting the element to its own layer, which stabilizes the
			  blend and prevents the color shift.
			-->
                        <button
                            bind:this={getStartedButton}
                            class="px-6 py-2.5 rounded-lg font-medium
                                   transition-colors duration-300 relative overflow-hidden
                                   hover:shadow-lg hover:shadow-primary/30
                                   focus:outline-none focus:ring-2 focus:ring-primary/50
                                   active:scale-[0.97] will-change-transform border"
                            style="background-color: rgba(159, 110, 247, var(--style-welcome-button-bg-opacity, 0.7));
                                   border-color: rgba(159, 110, 247, var(--style-welcome-button-border-opacity, 0.5));
                                   color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))"
                            on:click={handleNext}
                            on:mouseenter={handleMouseEnter}
                            on:mouseleave={handleMouseLeave}>
                            {buttonText}
                        </button>
                    </div>
                {/if}
            </div>
        </div>
    </div>
</div>

<style>
    /* Button ripple effect styles */
    button {
        position: relative;
        overflow: hidden;
    }
    
    /* Waving hand animation */
    .waving-hand {
        display: inline-block;
        transform-origin: 70% 70%;
        animation: wave 4s ease-in-out infinite;
    }
    
    @keyframes wave {
        0%, 80%, 100% {
            transform: rotate(0deg);
        }
        5%, 15% {
            transform: rotate(14deg);
        }
        10% {
            transform: rotate(-8deg);
        }
        20% {
            transform: rotate(14deg);
        }
        25% {
            transform: rotate(-4deg);
        }
    }
    
    /* Fade and drop animation */
    @keyframes fade-drop {
        from {
            opacity: 0;
            transform: translateY(30px);
        }
        to {
            opacity: 1;
            transform: translateY(0);
        }
    }
    
    .animate-fade-drop {
        animation: fade-drop 0.6s ease-out forwards;
    }
    
    /* 3D perspective for slide projector effect */
    :global(.slide-container) {
        perspective: 1000px;
        transform-style: preserve-3d;
    }
    
    /* Subtle skeleton sweep animation */
    @keyframes skeleton-sweep {
        0% { 
            transform: translateX(-100%);
        }
        100% { 
            transform: translateX(100%);
        }
    }
    
    /* Blinking cursor for typewriter effect */
    .animate-blink {
        animation: blink 1s infinite;
    }
    
    @keyframes blink {
        0%, 50% { opacity: 1; }
        51%, 100% { opacity: 0; }
    }
    
    /* Panel glow effect with smooth transitions */
    .panel-glow {
        position: relative;
        box-shadow: 0 20px 50px -12px rgba(0, 0, 0, 0.8);
        transition: box-shadow 2s cubic-bezier(0.4, 0, 0.2, 1);
    }
    
    .panel-glow:hover {
        box-shadow: 
            /* Right side glow - matching purple tones from GlowEffect */
            20px 15px 45px -10px rgba(159, 110, 247, 0.2),
            /* Bottom glow - stronger, closer to light source */
            0 30px 60px -15px rgba(190, 120, 255, 0.25),
            /* Bottom-right corner accent - where light hits directly */
            15px 25px 40px -12px rgba(255, 100, 255, 0.15),
            /* Original dark shadow */
            0 20px 50px -12px rgba(0, 0, 0, 0.8);
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
    
    /* Smooth fade mask for scrollable content */
    .mask-fade {
        mask-image: linear-gradient(
            to bottom,
            transparent,
            black 7%,
            black 93%,
            transparent
        );
        -webkit-mask-image: linear-gradient(
            to bottom,
            transparent,
            black 7%,
            black 93%,
            transparent
        );
        scrollbar-gutter: stable;
        scroll-behavior: smooth;
        -webkit-overflow-scrolling: touch;
        overscroll-behavior: contain;
        scrollbar-width: none;  /* Firefox */
        -ms-overflow-style: none;  /* IE and Edge */
    }

    /* Hide scrollbar completely */
    .mask-fade::-webkit-scrollbar {
        display: none;  /* Chrome, Safari, Opera */
    }
</style>