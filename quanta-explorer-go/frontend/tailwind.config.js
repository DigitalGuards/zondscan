/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './pages/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: {
          dark: '#1a1a1a'
        },
        secondary: {
          dark: '#2d2d2d'
        },
        accent: '#ffa729',
        text: {
          dark: '#ffffff',
          darkSecondary: '#a0aec0'
        }
      }
    },
  },
  plugins: [],
}
