/** @type {import('tailwindcss').Config} */

import colors from 'tailwindcss/colors';

// Define base color values
const primaryHue = 261;
const primarySaturation = '90%';
const primaryLightness = '70%';

const secondaryHue = 323;
const secondarySaturation = '100%';
const secondaryLightness = '72%';

const violet = `hsl(${primaryHue}, ${primarySaturation}, ${primaryLightness})`;
const pink = `hsl(${secondaryHue}, ${secondarySaturation}, ${secondaryLightness})`;
const red = colors.red[500];
const orange = colors.orange[500]; // Changed from yellow to orange
const green = '#68e796';
const yellow = '#fff38e';

export default {
  darkMode: 'class',
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
      	text: colors.gray[300],
        unobtrusive: colors.gray[400],
        bg: 'hsl(0, 0%, 7%)',
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
        
        'log-debug': violet,
        'log-info': green,
        'log-warn': yellow,
        'log-error': red,
        
        'error-task': orange,     // Orange for task errors (changed from yellow)
        'error-all': red,         // Red for critical errors
        'user-cancel': '#6b7280', // Gray for user cancellations
      },
      fontFamily: {
        sans: ['"DM Sans"', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
        mono: ['"DM Mono"', 'monospace']
      },
      // Animation durations and types
      animation: {
        // Controls the sweep animation speed (2s = moderate pace, lower = faster)
        'sweep-gradient': 'sweep var(--sweep-duration, 2s) linear infinite',
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
        // Defines how the sweeping gradient moves across the progress bar
        sweep: {
          // Start position (left edge of gradient is offscreen to the left)
          '0%': { 
            backgroundPosition: '0% 0',
          },
          // End position (gradient has moved completely across and is offscreen to the right)
          // The 200% value ensures the gradient completes a full sweep
          '100%': { 
            backgroundPosition: '200% 0',
          },
        },
      },
      /**
       * Progress bar animation configurations - these values control the appearance of the progress bars
       * across the application. Adjusting these values will affect all progress animations.
       */
      
      // Progress bar sweep gradient configuration
      backgroundImage: {
        // The gradient that forms the sweep effect on progress bars
        // - First value (0.05): Edge gradient opacity - lower for subtle edges
        // - Second value (0.3): Peak gradient brightness - determines the intensity of the sweep highlight
        'sweep-gradient': 'linear-gradient(90deg, transparent 0%, rgba(255,255,255,var(--sweep-edge-opacity, 0.05)) 25%, rgba(255,255,255,var(--sweep-peak-opacity, 0.3)) 50%, rgba(255,255,255,var(--sweep-edge-opacity, 0.05)) 75%, transparent 100%)',
      }
    },
  },
  plugins: [],
}