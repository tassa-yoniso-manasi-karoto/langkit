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
      }
    },
  },
  plugins: [],
}
