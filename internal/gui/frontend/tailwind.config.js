/** @type {import('tailwindcss').Config} */

import colors from 'tailwindcss/colors';

const violet = '#9f6ef7'
const red = colors.red[500]
const orange = colors.orange[500] // Changed from yellow to orange
const green = '#68e796'
const yellow = '#fff38e'
const pink = '#ff6ec7'

export default {
  darkMode: 'class',
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
      	text: colors.gray[300],
        unobtrusive: colors.gray[400],
        bg: '#1a1a1a',        // classic dark gray
        
        primary: violet,      // soft violet
        secondary: '#7851a9', // royal purple, darker than soft violet
        pink: pink,           // complementary pink for gradients
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
