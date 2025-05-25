/** @type {import('tailwindcss').Config} */

import colors from 'tailwindcss/colors';

// Define base color values
const primaryHue = 261;
const primarySaturation = '90%';
const primaryLightness = '70%';

const secondaryHue = 323;
const secondarySaturation = '100%';
const secondaryLightness = '72%';

// Core base colors as HSL
const violet = `hsl(${primaryHue}, ${primarySaturation}, ${primaryLightness})`;
const pink = `hsl(${secondaryHue}, ${secondarySaturation}, ${secondaryLightness})`;

// Error state colors in HSL format
const errorTaskHue = 50; // Yellow hue for task errors
const errorTaskSaturation = '90%';
const errorTaskLightness = '75%';

const errorAllHue = 0; // Red hue for critical errors
const errorAllSaturation = '100%';
const errorAllLightness = '45%';

const userCancelHue = 220; // Blue-gray hue for cancellations
const userCancelSaturation = '10%';
const userCancelLightness = '45%';

const completionHue = 130; // Green
const completionSaturation = '75%';
const completionLightness = '48%';

// UI Element Background colors
const uiElementHue = 0; // Neutral gray hue
const uiElementSaturation = '0%';
const uiElementLightness = '100%';
const uiElementAlpha = '0.05'; // Base opacity
const uiElementHoverAlpha = '0.08'; // Hover state opacity

// Define color values
const red = `hsl(${errorAllHue}, ${errorAllSaturation}, ${errorAllLightness})`;
const green = '#68e796';
const yellow = `hsl(${errorTaskHue}, ${errorTaskSaturation}, ${errorTaskLightness})`;
const userCancelGray = `hsl(${userCancelHue}, ${userCancelSaturation}, ${userCancelLightness})`;
const completionGreen = `hsl(${completionHue}, ${completionSaturation}, ${completionLightness})`;
const uiElementBg = `hsla(${uiElementHue}, ${uiElementSaturation}, ${uiElementLightness}, ${uiElementAlpha})`;
const uiElementHoverBg = `hsla(${uiElementHue}, ${uiElementSaturation}, ${uiElementLightness}, ${uiElementHoverAlpha})`;

export default {
  darkMode: 'class',
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
      	text: colors.gray[300],
        unobtrusive: colors.gray[400],
        logbg: 'hsl(0, 0%, 15%)',
        
        bgold: {
          50: 'hsl(0, 0%, 95%)',
          100: 'hsl(0, 0%, 90%)',
          200: 'hsl(0, 0%, 80%)',
          300: 'hsl(0, 0%, 70%)',
          400: 'hsl(0, 0%, 60%)',
          500: 'hsl(0, 0%, 50%)',
          600: 'hsl(0, 0%, 40%)',
          700: 'hsl(0, 0%, 30%)',
          800: 'hsl(0, 0%, 20%)',
          900: 'hsl(0, 0%, 10%)',
          950: 'hsl(0, 0%, 5%)',
          DEFAULT: 'hsl(0, 0%, 7%)',
        },
        primary: {
          50: `hsl(${primaryHue}, 92%, 95%)`,
          100: `hsl(${primaryHue}, 88%, 90%)`,
          200: `hsl(${primaryHue}, ${primarySaturation}, 80%)`,
          300: `hsl(${primaryHue}, ${primarySaturation}, 70%)`,
          400: `hsl(${primaryHue}, ${primarySaturation}, 60%)`,
          500: `hsl(${primaryHue}, ${primarySaturation}, 50%)`,
          600: `hsl(${primaryHue}, ${primarySaturation}, 40%)`,
          700: `hsl(${primaryHue}, ${primarySaturation}, 30%)`,
          800: `hsl(${primaryHue}, ${primarySaturation}, 20%)`,
          900: `hsl(${primaryHue}, 88%, 10%)`,
          950: `hsl(${primaryHue}, 92%, 5%)`,
          DEFAULT: violet,
        },
        secondary: {
          50: `hsl(${secondaryHue}, ${secondarySaturation}, 95%)`,
          100: `hsl(${secondaryHue + 1}, ${secondarySaturation}, 90%)`,
          200: `hsl(${secondaryHue}, ${secondarySaturation}, 80%)`,
          300: `hsl(${secondaryHue}, ${secondarySaturation}, 70%)`,
          400: `hsl(${secondaryHue}, ${secondarySaturation}, 60%)`,
          500: `hsl(${secondaryHue}, ${secondarySaturation}, 50%)`,
          600: `hsl(${secondaryHue}, ${secondarySaturation}, 40%)`,
          700: `hsl(${secondaryHue}, ${secondarySaturation}, 30%)`,
          800: `hsl(${secondaryHue}, ${secondarySaturation}, 20%)`,
          900: `hsl(${secondaryHue + 1}, ${secondarySaturation}, 10%)`,
          950: `hsl(${secondaryHue}, ${secondarySaturation}, 5%)`,
          DEFAULT: pink,
        },
        tertiary: '#7851a9', // royal purple, darker than soft violet, 
        pink: pink, // FIXME should be depreciated in favor of secondary
        
        
        'error-card-bg': '#281937',    // slightly purplish dark background for error cards
        'error-card-hover': '#321e41', // slightly lighter for hover state
        'tooltip-bg': '#1c1c24',        // dark background for tooltips
        'tooltip-border': '#3b3167',    // border color for tooltips
        
        // TODO cleanup bg, component bg etc
        'ui-element': uiElementBg,      // subtle background for UI elements like MediaInput and ProgressManager
        'ui-element-hover': uiElementHoverBg, // hover state for UI elements
        'input-bg': 'var(--input-bg)',
        
        accent: {
          50: '#fffce5',
          100: '#fffacc',
          200: '#fff599',
          300: '#fff066',
          400: '#ffeb33',
          500: '#ffe500',
          600: '#ccb800',
          700: '#998a00',
          800: '#665c00',
          900: '#332e00',
          950: '#1a1700',
         },
        'pale-green': green,
        'welcome-gradient-start': `hsl(${primaryHue}, 95%, 75%)`,
        'welcome-gradient-end': `hsl(${secondaryHue}, 95%, 70%)`,
        
        'log-debug': violet,
        'log-info': green,
        'log-warn': yellow,
        'log-error': red,
        
        // Error states with HSL definitions
        'error-soft': yellow,     // Yellow for task errors
        'error-soft-hue': errorTaskHue,
        'error-soft-saturation': errorTaskSaturation,
        'error-soft-lightness': errorTaskLightness,
        
        'error-hard': red,         // Red for critical errors
        'error-hard-hue': errorAllHue,
        'error-hard-saturation': errorAllSaturation,
        'error-hard-lightness': errorAllLightness,
        
        'user-cancel': userCancelGray, // Gray for user cancellations
        'user-cancel-hue': userCancelHue,
        'user-cancel-saturation': userCancelSaturation,
        'user-cancel-lightness': userCancelLightness,
        
        'completion': completionGreen,
        
        // Feature group colors
        'group-subtitle': 'hsla(210, 90%, 60%, 0.35)',  // Blue for subtitle group
        'group-merge': 'hsla(130, 90%, 50%, 0.35)'  // Green for merge/output group
      },
      fontFamily: {
        sans: ['"DM Sans"', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
        mono: ['"DM Mono"', 'monospace']
      },
      // Animation durations and types
      animation: {
        // Controls the sweep animation speed (2s = moderate pace, lower = faster)
        'sweep-gradient': 'sweep var(--sweep-duration, 2s) linear infinite',
        // Feature message fade in animation
        'fadeIn': 'fadeIn 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
        // Subtle pulse animation for message items hover
        'subtlePulse': 'subtlePulse 2s ease-in-out infinite',
        // Individual message item fade in animation
        'messageIn': 'messageIn 0.3s cubic-bezier(0.16, 1, 0.3, 1) both',
      },
      
      // Shadow effects for various UI elements
      boxShadow: {
        // Shadow at the progress edge to create a subtle glow at the boundary
        // - Blur radius (4px): Controls how soft/diffuse the glow appears
        // - Spread radius (1px): Controls how far the glow extends
        // - Opacity (0.4): Controls how transparent the glow is (lower = more subtle)
        'progress-edge': '0 0 var(--progress-edge-blur, 4px) var(--progress-edge-spread, 1px) rgba(159,110,247,var(--progress-edge-opacity, 0.4))',
      },
      // Animation keyframes definitions
      keyframes: {
        // Fade in animation for feature messages
        fadeIn: {
          '0%': {
            opacity: '0',
            transform: 'translateY(-5px) scale(0.98)'
          },
          '100%': {
            opacity: '1',
            transform: 'translateY(0) scale(1)'
          }
        },
        // Subtle pulse animation for icons in feature messages
        subtlePulse: {
          '0%, 100%': {
            opacity: '1',
            transform: 'scale(1)'
          },
          '50%': {
            opacity: '0.85',
            transform: 'scale(0.97)'
          }
        },
        // Animation for individual feature message items
        messageIn: {
          '0%': {
            opacity: '0',
            transform: 'translateY(-2px)'
          },
          '100%': {
            opacity: '1',
            transform: 'translateY(0)'
          }
        },
      },
    },
  },
  plugins: [],
}