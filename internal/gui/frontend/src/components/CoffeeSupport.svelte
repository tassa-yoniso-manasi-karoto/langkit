<script>
    import { OpenURL } from '../../wailsjs/go/gui/App';
    import { statisticsStore } from '../lib/stores';
    
    export let version = '';
    export let github_url = 'https://github.com/tassa-yoniso-manasi-karoto/langkit/#support-the-project';
    
    const handleClick = (event) => {
        event.preventDefault();
        OpenURL(github_url);
    };

    $: isVisible = ($statisticsStore?.countAppStart > 20 && $statisticsStore?.countProcessStart > 30) || version === 'dev';
</script>

{#if isVisible}
    <a 
        href={github_url}
        on:click={handleClick}
        class="ml-2 inline-flex items-center cursor-pointer hover:opacity-80 transition-opacity duration-200"
        title="Support the project!">
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" class="coffee-icon h-7 w-7">
            <defs>
                <mask id="steamWispMotionMask">
                    <g stroke="#fff" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5">
                        <path d="M12 0c0 2-1 2-1 4s1 2 1 4s-1 2-1 4s1 2 1 4">
                            <animateMotion calcMode="linear" dur="3s" path="M0 0v-8" repeatCount="indefinite"/>
                        </path>
                        <path d="M8 0c0 2-1 2-1 4s1 2 1 4s-1 2-1 4s1 2 1 4">
                             <animateMotion calcMode="linear" dur="3s" path="M0 0v-8" begin="0.15s" repeatCount="indefinite"/>
                        </path>
                        <path d="M16 0c0 2-1 2-1 4s1 2 1 4s-1 2-1 4s1 2 1 4">
                             <animateMotion calcMode="linear" dur="3s" path="M0 0v-8" begin="0.3s" repeatCount="indefinite"/>
                        </path>
                    </g>
                </mask>

                <linearGradient id="steamColorWithTopFade" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#D1D5DB" stop-opacity="0" />
                    <stop offset="33%" stop-color="#D1D5DB" stop-opacity="0.3" />
                    <stop offset="66%" stop-color="#D1D5DB" stop-opacity="0.5" />
                    <stop offset="100%" stop-color="#D1D5DB" stop-opacity="0.8" />
                </linearGradient>

                <linearGradient id="coffeeFillGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stop-color="#794D22" />
                    <stop offset="100%" stop-color="#503315" />
                </linearGradient>
            </defs>

            <g id="coffeeIconInteractiveGroup">
                <rect id="steamRect" x="0" width="24" 
                      fill="url(#steamColorWithTopFade)" 
                      mask="url(#steamWispMotionMask)"
                      height="0" 
                      y="7.5">
                    <animate attributeName="y" id="steamIntroY" dur="0.6s" values="7.5;0.5" fill="freeze" begin="0.2s"/>
                    <animate attributeName="height" id="steamIntroHeight" dur="0.6s" values="0;7" fill="freeze" begin="0.2s"/>
                </rect>

                <g id="mugGroup" fill="none" stroke="var(--style-coffee-mug-color, #6B7280)" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                    <path id="mugBody" fill="url(#coffeeFillGradient)" fill-opacity="0" 
                          stroke-dasharray="36" stroke-dashoffset="36"
                          d="M16.5 9v5c0 1.66 -1.34 3 -3 3h-5c-1.66 0 -3 -1.34 -3 -3v-5Z"> 
                        <animate attributeName="stroke-dashoffset" dur="0.6s" values="36;0" fill="freeze"/>
                        <animate attributeName="fill-opacity" begin="0.7s" dur="0.15s" values="0;0.8" fill="freeze"/>
                    </path>
                    <path id="mugHandle" stroke-dasharray="14" stroke-dashoffset="14"
                          d="M16.5 9h3c0.55 0 1 0.45 1 1v3c0 0.55 -0.45 1 -1 1h-3">
                        <animate attributeName="stroke-dashoffset" begin="0.6s" dur="0.2s" values="14;0" fill="freeze"/>
                    </path>
                </g>
            </g>
        </svg>
    </a>
{/if}

<style>
    .coffee-icon {
        filter: drop-shadow(0 0 2px rgba(121, 77, 34, 0.3));
        transition: filter 0.3s ease, opacity 0.2s; 
    }
    
    a:hover .coffee-icon {
        filter: drop-shadow(0 0 4px rgba(121, 77, 34, 0.5));
    }

    /* Define the keyframe animation for hover */
    @keyframes coffeeHoverEffect {
        0% {
            transform: rotate(0deg) scale(1);
        }
        50% { 
            /* Optional: add an intermediate step for a more dynamic feel */
            /* Example: transform: rotate(-2deg) scale(1.02); */
            transform: rotate(3.5deg) scale(1.04); /* Halfway to target, slight overshoot on scale */
        }
        100% {
            transform: rotate(7deg) scale(1.07);
        }
    }

    #coffeeIconInteractiveGroup {
        transform-origin: 50% 75%; /* Center horizontally, 75% from the top vertically */
        /* Base transition for when hover ends, or for non-animated properties */
        transition: transform 0.3s ease-out; 
    }

    a:hover .coffee-icon #coffeeIconInteractiveGroup {
        /* Apply the keyframe animation on hover */
        animation-name: coffeeHoverEffect;
        animation-duration: 0.2s; /* Duration of one cycle of the hover animation */
        animation-timing-function: ease-in-out;
        animation-fill-mode: forwards; /* Retain the state of the last keyframe when animation ends */
        /* transform: rotate(7deg) scale(1.05); /* Fallback or if animation is not supported, but animation-fill-mode: forwards handles this */
    }

    a:active .coffee-icon #coffeeIconInteractiveGroup {
        /* Click effect overrides hover animation if it's playing */
        animation-name: none; /* Stop the hover animation if it was playing */
        transform: rotate(3deg) scale(1.1); 
        transition-duration: 0.1s; /* Faster transition for active state, overrides base transition */
    }
</style>