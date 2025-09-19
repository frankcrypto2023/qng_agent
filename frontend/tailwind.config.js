/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        blockchain: {
          primary: '#6366f1',
          secondary: '#8b5cf6',
          accent: '#06b6d4',
          dark: '#1e293b',
          darker: '#0f172a',
        }
      },
      backgroundImage: {
        'blockchain-gradient': 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
        'chat-gradient': 'linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%)',
      }
    },
  },
  plugins: [],
}