<script lang="ts">
    import { onMount } from 'svelte';
    import { fade, scale, fly } from 'svelte/transition';
    import { cubicOut, backOut, elasticOut } from 'svelte/easing';
    import { CheckDockerAvailability, CheckInternetConnectivity } from '../../wailsjs/go/gui/App';
    import { statisticsStore } from '../lib/stores';
    import { logger } from '../lib/logger';
    import DockerUnavailableIcon from './icons/DockerUnavailableIcon.svelte';
    
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
    let dockerStatus: any = null;
    let internetStatus: any = null;
    let dockerReady = false;
    let internetReady = false;
    let showWelcome = true;
    let showApiKeys = false;
    
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
        fill.style.backgroundColor = 'hsla(320, 90%, 60%, 0.9)'; // Vibrant pink-purple for visibility
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
    
    // Check statuses on mount
    onMount(async () => {
        logger.info('WelcomePopup', 'Welcome popup mounted');
        
        // Start reveal animation sequence
        setTimeout(() => titleVisible = true, 100);
        setTimeout(() => contentVisible = true, 300);
        setTimeout(() => actionsVisible = true, 500);
        
        // Start checking statuses after a brief delay
        setTimeout(() => checkStatuses(), 600);
    });
    
    async function checkStatuses() {
        // Check Docker with delay for smooth animation
        setTimeout(async () => {
            try {
                dockerStatus = await CheckDockerAvailability();
                logger.debug('WelcomePopup', 'Docker check completed', dockerStatus);
            } catch (error) {
                logger.error('WelcomePopup', 'Docker check failed', { error });
                dockerStatus = { available: false, error: 'Check failed' };
            } finally {
                dockerReady = true;
            }
        }, 200);
        
        // Check Internet with staggered delay
        setTimeout(async () => {
            try {
                internetStatus = await CheckInternetConnectivity();
                logger.debug('WelcomePopup', 'Internet check completed', internetStatus);
            } catch (error) {
                logger.error('WelcomePopup', 'Internet check failed', { error });
                internetStatus = { online: false, error: 'Check failed' };
            } finally {
                internetReady = true;
            }
        }, 600);
    }
    
    function handleNext() {
        if (showWelcome) {
            logger.info('WelcomePopup', 'Moving to API keys explanation');
            showWelcome = false;
            // Start showing API keys almost immediately to create overlap effect
            setTimeout(() => {
                showApiKeys = true;
                currentStep = 1;
            }, 50);
        } else if (showApiKeys) {
            logger.info('WelcomePopup', 'User clicked Get Started');
            onClose();
        }
    }
    
    function goToPage(page: number) {
        if (page === 0 && !showWelcome) {
            logger.info('WelcomePopup', 'Switching to Welcome page');
            showApiKeys = false;
            setTimeout(() => {
                showWelcome = true;
                currentStep = 0;
            }, 50);
        } else if (page === 1 && !showApiKeys) {
            logger.info('WelcomePopup', 'Switching to API Keys page');
            showWelcome = false;
            setTimeout(() => {
                showApiKeys = true;
                currentStep = 1;
            }, 50);
        }
    }
    
    $: buttonText = showWelcome ? 'Continue' : 'Get Started';
</script>

<div class="fixed inset-0 z-[9999] flex items-center justify-center p-4 backdrop-blur-sm"
     in:fade={{ duration: 300 }}
     out:fade={{ duration: 200 }}>
    
    <!-- Background overlay -->
    <div class="absolute inset-0 bg-black/40" on:click={onClose}></div>
    
    <!-- Popup container with fixed size -->
    <div class="relative max-w-2xl w-full"
         in:scale={{ duration: 400, easing: backOut, start: 0.95 }}
         out:scale={{ duration: 200, easing: cubicOut, start: 0.95 }}>
        
        <!-- Panel with more solid appearance and depth -->
        <div class="relative overflow-hidden rounded-3xl
                    bg-black/30 backdrop-blur-2xl
                    border border-white/10
                    shadow-[0_20px_50px_-12px_rgba(0,0,0,0.8)]">
            
            <!-- Subtle gradient accent -->
            <div class="absolute -top-32 -right-32 w-64 h-64 rounded-full bg-primary/10 blur-3xl"></div>
            
            <!-- Content -->
            <div class="relative p-8 md:p-12">
                <!-- Content container with 3D slide effect -->
                <div class="min-h-[450px] relative slide-container overflow-hidden">
                    <!-- Step 0: Welcome page -->
                    {#if showWelcome}
                        <div class="absolute inset-0 flex flex-col items-center"
                             in:slideProjector={{ yStart: 100, yEnd: 0, scaleStart: 0.8, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, duration: 800 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            <!-- Welcome header -->
                            <div class="text-center mb-8 pt-4">
                                <h1 class="text-4xl md:text-5xl font-bold text-white mb-3 flex items-center justify-center gap-2">
                                    Welcome to Langkit
                                    <span class="waving-hand text-4xl md:text-5xl">👋</span>
                                </h1>
                                <p class="text-lg text-white/70">
                                    Let's check your system requirements
                                </p>
                            </div>
                            
                            <!-- System requirements cards -->
                            <div class="space-y-6 px-4 w-full max-w-xl">
                        <!-- Docker Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        bg-white/5 backdrop-blur-sm border border-white/10
                                        transition-all duration-300 hover:bg-white/10
                                        relative overflow-hidden"
                                 in:fly={{ y: 20, duration: 400, delay: 50, easing: cubicOut }}>
                                
                                {#if !dockerReady}
                                    <!-- Skeleton loader overlay -->
                                    <div class="absolute inset-0 animate-skeleton-sweep"></div>
                                {/if}
                                
                                <div class="flex items-center gap-3 relative">
                                    <div class="w-8 h-8 flex items-center justify-center">
                                        {#if dockerReady}
                                            {#if dockerStatus?.available}
                                                <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" class="text-primary">
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
                                                <DockerUnavailableIcon size="32" className="text-yellow-500" />
                                            {:else}
                                                <span class="material-icons text-3xl text-gray-400">pending</span>
                                            {/if}
                                        {:else}
                                            <div class="w-8 h-8 rounded-full bg-white/10"></div>
                                        {/if}
                                    </div>
                                    <div>
                                        {#if dockerReady}
                                            <h3 class="text-white font-medium">Docker Desktop</h3>
                                            <p class="text-sm text-white/60">
                                                {#if dockerStatus?.available}
                                                    Version {dockerStatus.version || 'detected'}
                                                {:else if dockerStatus?.error}
                                                    {dockerStatus.error}
                                                {:else}
                                                    Checking availability...
                                                {/if}
                                            </p>
                                        {:else}
                                            <div class="h-5 bg-white/10 rounded w-32 mb-1"></div>
                                            <div class="h-3.5 bg-white/5 rounded w-48"></div>
                                        {/if}
                                    </div>
                                </div>
                                
                                {#if dockerReady && dockerStatus && !dockerStatus.available}
                                    <a href="https://docs.docker.com/get-docker/" 
                                       target="_blank"
                                       class="text-primary hover:text-primary/80 transition-colors">
                                        <span class="material-icons text-sm">open_in_new</span>
                                    </a>
                                {/if}
                            </div>
                            
                            {#if dockerReady && dockerStatus && !dockerStatus.available}
                                <div class="px-4 py-3 rounded-xl bg-yellow-500/10 border border-yellow-500/20"
                                     in:fade={{ duration: 300 }}>
                                    <p class="text-sm text-yellow-200/80">
                                        Without Docker, some advanced processing features will be unavailable.
                                        Voice separation and certain subtitle operations require Docker to function.
                                    </p>
                                </div>
                            {/if}
                        </div>
                        
                        <!-- Internet Status -->
                        <div class="space-y-3">
                            <div class="flex items-center justify-between p-4 rounded-2xl
                                        bg-white/5 backdrop-blur-sm border border-white/10
                                        transition-all duration-300 hover:bg-white/10
                                        relative overflow-hidden"
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
                                            <h3 class="text-white font-medium">Internet Connection</h3>
                                            <p class="text-sm text-white/60">
                                                {#if internetStatus?.online}
                                                    Connected ({internetStatus.latency}ms latency)
                                                {:else if internetStatus?.error}
                                                    {internetStatus.error}
                                                {:else}
                                                    Checking connectivity...
                                                {/if}
                                            </p>
                                        {:else}
                                            <div class="h-5 bg-white/10 rounded w-32 mb-1"></div>
                                            <div class="h-3.5 bg-white/5 rounded w-48"></div>
                                        {/if}
                                    </div>
                                </div>
                            </div>
                            
                            {#if internetReady && internetStatus && !internetStatus.online}
                                <div class="px-4 py-3 rounded-xl bg-red-500/10 border border-red-500/20"
                                     in:fade={{ duration: 300 }}>
                                    <p class="text-sm text-red-200/80">
                                        An internet connection is required for AI-powered features.
                                        Translation and voice synthesis services will not be available offline.
                                    </p>
                                </div>
                            {/if}
                            </div>
                            </div>
                        </div>
                    {/if}
                    
                    <!-- Step 1: API Keys page -->
                    {#if showApiKeys}
                        <div class="absolute inset-0 flex flex-col items-center justify-center"
                             in:slideProjector={{ yStart: 150, yEnd: 0, scaleStart: 0.65, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, blurStart: 6, blurEnd: 0, duration: 800, delay: 0 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            
                            <div class="text-center max-w-lg mx-auto px-4">
                                <!-- API Keys header grouped with content -->
                                <h2 class="text-3xl font-semibold text-white mb-6 flex items-center justify-center gap-2">
                                    <span class="material-icons text-primary/70">vpn_key</span>
                                    Understanding API Keys
                                </h2>
                                
                                <div class="space-y-4">
                                    <p class="text-base text-white/80 leading-relaxed">
                                        API keys are secure codes that allow Langkit to access cloud-based AI services.
                                    </p>
                                    
                                    <p class="text-base text-white/70 leading-relaxed">
                                        They enable powerful features like speech-to-text and audio enhancement 
                                        without requiring expensive local hardware.
                                    </p>
                                    
                                    <p class="text-sm text-white/50 italic mt-6">
                                        Think of them as membership cards for premium AI services
                                    </p>
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
                                class="w-2 h-2 rounded-full transition-all duration-300 {showWelcome ? 'bg-primary' : 'bg-white/30'}"
                                on:click={() => goToPage(0)}>
                            </div>
                            <div 
                                class="w-2 h-2 rounded-full transition-all duration-300 {showApiKeys ? 'bg-primary' : 'bg-white/30'}"
                                on:click={() => goToPage(1)}>
                            </div>
                        </div>
                        <button
                            bind:this={getStartedButton}
                            class="px-6 py-2.5 rounded-lg bg-primary/70 text-white font-medium
                                   transition-colors duration-300 relative overflow-hidden
                                   hover:shadow-lg hover:shadow-primary/30
                                   active:scale-[0.98] focus:outline-none focus:ring-2 focus:ring-primary/50
                                   border border-primary/50"
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
    
    button div {
        pointer-events: none;
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