/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Background colors
        background: {
          DEFAULT: '#1a1a1a',    // Primary background
          secondary: '#2d2d2d',  // Cards, elevated surfaces
          tertiary: '#1f1f1f',   // Gradient end, subtle variation
        },
        // Border colors
        border: {
          DEFAULT: '#3d3d3d',    // Standard borders
          hover: '#4d4d4d',      // Border on hover
        },
        // Accent colors (QRL orange)
        accent: {
          DEFAULT: '#ffa729',    // Primary accent
          hover: '#ffb954',      // Lighter for hover states
          dark: '#ff9709',       // Darker for pressed states
        },
        // Text colors
        text: {
          primary: '#ffffff',    // Primary text
          secondary: '#a0aec0',  // Muted text (gray-400 equivalent)
          muted: '#9ca3af',      // More muted (gray-400)
        },
        // Legacy aliases for backwards compatibility
        primary: {
          dark: '#1a1a1a'
        },
        secondary: {
          dark: '#2d2d2d'
        },
      },
      // Common gradient backgrounds
      backgroundImage: {
        'card-gradient': 'linear-gradient(to bottom right, #2d2d2d, #1f1f1f)',
        'sidebar-gradient': 'linear-gradient(to bottom, #1a1a1a, #1a1a1a, #1f1f1f)',
      },
      // Box shadows for cards
      boxShadow: {
        'card': '0 10px 15px -3px rgba(0, 0, 0, 0.3), 0 4px 6px -2px rgba(0, 0, 0, 0.15)',
        'sidebar': '4px 0 24px rgba(0, 0, 0, 0.2)',
      },
    },
  },
  plugins: [],
}
