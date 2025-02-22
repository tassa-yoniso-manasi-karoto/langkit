/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        'accent': '#9f6ef7',    // soft violet
        'bg': '#1a1a1a',      // Classic dark gray
      },
      fontFamily: {
        'sans': ['"DM Sans"', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
        'mono': ['"DM Mono"', 'monospace']
      }
    },
  },
  plugins: [],
};
