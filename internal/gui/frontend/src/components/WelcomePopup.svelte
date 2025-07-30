<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fade, scale, fly, slide } from 'svelte/transition';
    import { cubicOut, backOut, elasticOut } from 'svelte/easing';
    import lottie from 'lottie-web';
    import { statisticsStore, systemInfoStore } from '../lib/stores';
    import { logger } from '../lib/logger';
    import DependenciesChecklist from './DependenciesChecklist.svelte';
    import { OpenURL as BrowserOpenURL } from '../api/services/system';
    
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
    export let recheckFFmpeg: () => Promise<void>;
    export let recheckMediaInfo: () => Promise<void>;
    
    // State variables
    let showWelcome = true;
    let showApiKeys = false;
    let showLanguages = false;
    
    // Pure CSS approach using clamp for responsive sizing
    const containerMinHeight = 'clamp(450px, 60vh, 630px)';
    const containerMaxHeight = 'min(calc(100vh - 140px), 800px)';
    
    // Get system info from store
    $: systemInfo = $systemInfoStore;
    
    // Animation states
    let titleVisible = false;
    let contentVisible = false;
    let actionsVisible = false;
    
    // Step management
    let currentStep = 0; // 0: requirements, 1: api keys, 2: get started
    
    // Button ripple effect
    let getStartedButton: HTMLButtonElement | null = null;
    
    // Lottie animation
    let lottieContainer: HTMLDivElement;
    let lottieAnimation: any = null;
    let animationTimeout: ReturnType<typeof setTimeout> | null = null;
    let waveCount = 0;
    let isAnimationInitialized = false;
    
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
    function initializeLottieAnimation() {
    	if (!lottieContainer || isAnimationInitialized) return;
    	
    	// Clean up any existing animation first
    	cleanupLottieAnimation();
    	
    	lottieAnimation = lottie.loadAnimation({
    		container: lottieContainer,
    		renderer: 'svg',
    		loop: false,
    		autoplay: false,
    		path: '/waving-hand.json'
    	});
    	
    	lottieAnimation.setSpeed(1.35);
    	isAnimationInitialized = true;
    	waveCount = 0;
    	
    	// Add complete event listener for double wave behavior
    	lottieAnimation.addEventListener('complete', () => {
    		waveCount++;
    		if (waveCount < 2) {
    			// Play second wave immediately
    			lottieAnimation.goToAndPlay(0);
    		} else {
    			// Reset count and wait 4 seconds before next double wave
    			waveCount = 0;
    			animationTimeout = setTimeout(() => {
    				if (lottieAnimation) {
    					lottieAnimation.goToAndPlay(0);
    				}
    			}, 4000);
    		}
    	});
    	
    	// Initial 4-second delay before first animation
    	animationTimeout = setTimeout(() => {
    		if (lottieAnimation) {
    			lottieAnimation.play();
    		}
    	}, 4000);
    }
    
    function cleanupLottieAnimation() {
    	if (animationTimeout) {
    		clearTimeout(animationTimeout);
    		animationTimeout = null;
    	}
    	if (lottieAnimation) {
    		lottieAnimation.destroy();
    		lottieAnimation = null;
    	}
    	isAnimationInitialized = false;
    	waveCount = 0;
    }
    
    // Reactive statement to handle navigation back to welcome page
    $: if (showWelcome && lottieContainer) {
    	// Use a small delay to ensure DOM is fully ready
    	setTimeout(() => initializeLottieAnimation(), 50);
    } else if (!showWelcome) {
    	// Clean up when navigating away
    	cleanupLottieAnimation();
    }
    
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
    	
    	// Initialize Lottie animation on mount (will be called again reactively)
    	initializeLottieAnimation();
    	
    	// Add keyboard listener
    	window.addEventListener('keydown', handleKeydown);
    });
    
    onDestroy(() => {
        window.removeEventListener('keydown', handleKeydown);
        cleanupLottieAnimation();
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
    
    $: buttonText = showWelcome ? 'Continue' : showApiKeys ? 'Continue' : 'Get Started';
   </script>

<div class="welcome-popup fixed inset-0 flex items-center justify-center p-4 backdrop-blur-sm"
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
                <div class="relative slide-container overflow-x-hidden overflow-y-auto mask-fade" style="min-height: {containerMinHeight}; max-height: {containerMaxHeight};">
                    <!-- Step 0: Welcome page -->
                    {#if showWelcome}
                        <div class="absolute top-0 left-0 right-0 flex flex-col items-center pb-5"
                             in:slideProjector={{ yStart: 100, yEnd: 0, scaleStart: 0.8, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, duration: 800 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            <!-- Welcome header -->
                            <div class="text-center mb-8 pt-4">
                                <h1 class="text-5xl md:text-5xl font-[Outfit] font-bold mb-3 flex items-center justify-center gap-2"
                                    style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    Welcome to Langkit
                                    <div bind:this={lottieContainer} class="inline-block w-12 h-12 md:w-14 md:h-14"></div>
                                </h1>
                                <p class="text-lg min-h-[28px]"
                                   style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                    Let's check your system requirements
                                </p>
                            </div>
                            
                            <!-- System requirements checklist -->
                            <DependenciesChecklist {recheckFFmpeg} {recheckMediaInfo} />
                        </div>
                    {/if}
                    
                    <!-- Step 1: API Keys page -->
                    {#if showApiKeys}
                        <div class="absolute top-0 left-0 right-0 flex flex-col items-center justify-center pt-5 pb-5"
                             in:slideProjector={{ yStart: 150, yEnd: 0, scaleStart: 0.65, scaleEnd: 1, opacityStart: 0, opacityEnd: 1, blurStart: 6, blurEnd: 0, duration: 800, delay: 0 }}
                             out:slideProjectorOut={{ duration: 800 }}>
                            
                            <div class="text-center max-w-lg mx-auto px-4">
                                <!-- API Keys header grouped with content -->
                                <h2 class="text-4xl font-semibold mb-6 flex items-center justify-center gap-2"
                                    style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 32 32" class="text-primary" stroke-width="1" stroke="currentColor">
                                        <path fill="currentColor" d="M28 26c-.178 0-.347.03-.511.074l-1.056-1.055c.352-.595.567-1.28.567-2.019s-.215-1.424-.567-2.019l1.055-1.055c.165.043.334.074.512.074a2 2 0 1 0-2-2c0 .178.03.347.074.512l-1.055 1.055C24.424 19.215 23.739 19 23 19s-1.424.215-2.019.567l-1.055-1.055c.043-.165.074-.334.074-.512a2 2 0 1 0-2 2c.178 0 .347-.03.512-.074l1.055 1.055C19.215 21.576 19 22.261 19 23s.215 1.424.567 2.019l-1.055 1.055A2 2 0 0 0 18 26a2 2 0 1 0 2 2c0-.178-.03-.347-.074-.512l1.055-1.055c.595.352 1.28.567 2.019.567s1.424-.215 2.019-.567l1.055 1.055A2 2 0 0 0 26 28a2 2 0 1 0 2-2m-7-3c0-1.102.897-2 2-2s2 .898 2 2s-.897 2-2 2s-2-.897-2-2"/>
                                        <circle cx="22" cy="10" r="2" fill="currentColor"/>
                                        <path fill="currentColor" d="M21 2c-4.963 0-9 4.037-9 9c0 .779.099 1.547.294 2.291L2 23.586V30h6.414l7-7l-2.707-2.707l-1.414 1.414L12.586 23l-1.59 1.59l-1.287-1.295l-1.418 1.41l1.29 1.299L7.587 28H4v-3.586l9.712-9.712l.856-.867l-.199-.585A7.008 7.008 0 0 1 21 4c3.86 0 7.001 3.14 7.001 7h2c0-4.963-4.037-9-9-9Z"/>
                                    </svg>
                                    Understanding API Keys
                                </h2>
                                
                                <div class="space-y-4">
                                    <p class="text-lg leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        API keys are <strong>secure, confidential codes</strong> that allow Langkit to access cloud-based AI services.
                                    </p>
                                    <p class="text-lg leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        <strong>
                                            An API key is like your private electricity meter number that tracks how much power you use so the company can bill you accurately.
                                        </strong>
                                    </p>
                                    <p class="text-lg leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        <strong>
                                            Don't share it with anyone!
                                        </strong>
                                    </p>
                                    
                                    <p class="text-lg leading-relaxed"
                                       style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                        They enable powerful features like speech-to-text (making dubtites), subtitle summarization and voice enhancement at a low cost or for free without requiring expensive local hardware.
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
                                    <p class="text-lg leading-relaxed"
                                        style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        For languages with complex writing systems (like Japanese & Indic scripts), Langkit rely on external tools.
                                        These tools run inside a system called <strong>Docker Desktop</strong>.
                                    </p>
                                    <p class="text-lg leading-relaxed"
                                        style="color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                        Docker solves the problem of "this tool is too complicated for normal people to install". 
                                        Docker packages these nightmare-to-install tools into something that works out-of-the-box.
                                    </p>
                                    {#if systemInfo.os === 'windows'}
                                    <p class="text-lg leading-relaxed"
                                    	style="color: rgba(255, 255, 255, var(--style-welcome-text-secondary-opacity, 0.7))">
                                    	⚠️ In addition to installing Docker Desktop itself, Windows users require a one-time setup of the <strong>Windows Subsystem for Linux (WSL)</strong> for Docker to work.
                                    </p>
                                    <div class="pt-4">
                                    	<button on:click={() => BrowserOpenURL('https://docs.microsoft.com/en-us/windows/wsl/install')} class="px-6 py-2.5 rounded-lg font-medium transition-colors duration-300 relative overflow-hidden hover:shadow-lg hover:shadow-primary/30 focus:outline-none focus:ring-2 focus:ring-primary/50 active:scale-[0.97] will-change-transform border" style="background-color: rgba(159, 110, 247, var(--style-welcome-button-bg-opacity, 0.7)); border-color: rgba(159, 110, 247, var(--style-welcome-button-border-opacity, 0.5)); color: rgba(255, 255, 255, var(--style-welcome-text-primary-opacity, 1))">
                                    		View official WSL Installation Guide
                                    	</button>
                                    </div>
                                    {/if}
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