import './globals.css'
import Sidebar from "./components/Sidebar"
import AuthProvider from "./components/AuthProvider"

export const metadata = {
  title: 'QRL Explorer',
  description: 'Quantum Resistant Ledger Proof-of-Stake Blockchain Explorer',
  themeColor: '#1a1a1a'
}

interface RootLayoutProps {
  children: React.ReactNode
}

export default function RootLayout({ children }: RootLayoutProps) {
  return (
    <AuthProvider>
      <html lang="en" className="dark">
        <body className="min-h-screen bg-[#1a1a1a] text-gray-300">
          <div className="flex min-h-screen">
            <Sidebar />
            <main className="flex-1 ml-64 min-h-screen relative">
              <div className="absolute inset-0 bg-[url('/circuit-board.svg')] opacity-[0.02]"></div>
              <div className="relative">
                {children}
              </div>
            </main>
          </div>
        </body>
      </html>
    </AuthProvider>
  )
}
