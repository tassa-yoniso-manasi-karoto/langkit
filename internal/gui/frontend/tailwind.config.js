/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      colors: {
        'accent': '#9f6ef7',    // soft violet
        'accent-2': '#936dd4',    // dusty
        'accent-3': '#8657d1',    //rich
        'accent-4': '#7c4ec9',    //deep
        'accent-5': '#7347b8',    //royal
        
        // Alternative background colors you can try:
        'bg-1': '#0f172a',      // Current dark blue slate
        'bg-2': '#1a1625',      // Dark purple-black
        'bg-3': '#131515',      // Almost black with slight green tint
        'bg-4': '#1f1720',      // Dark burgundy-black
        'bg-5': '#161a26',      // Navy dark
        'bg': '#1a1a1a',      // Classic dark gray
      }
    },
  },
  plugins: [],
}

