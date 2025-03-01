/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        accent: '#9f6ef7',  // soft violet
        bg: '#1a1a1a',      // classic dark gray
        'error-task': '#f97316', // Orange for task errors
        'error-all': '#ef4444',  // Red for critical errors
        'user-cancel': '#6b7280', // Gray for user cancellations
        primary: {
          50: '#f5f3ff',
          100: '#ede9fe',
          200: '#ddd6fe',
          300: '#c4b5fd',
          400: '#a78bfa',
          500: '#8b5cf6',
          600: '#9f6ef7',
          700: '#7c3aed',
          800: '#6d28d9',
          900: '#5b21b6',
        },
      },
      fontFamily: {
        sans: ['"DM Sans"', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
        mono: ['"DM Mono"', 'monospace']
      }
    },
  },
  plugins: [],
}
