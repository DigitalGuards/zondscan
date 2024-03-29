import React from "react"
import type { Metadata } from 'next'
import Header from "./components/Header"
import Footer from "./components/Footer"

// These styles apply to every route in the application
import './globals.css'
import AuthProvider from "./components/AuthProvider"

export const metadata: Metadata = {
  title: 'QRL Explorer',
  description: 'Quantum Resistant Ledger Proof-of-Stake Blockchain Explorer',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthProvider>
    <html lang="en">
      <body>
        <Header />
        {children}
        <Footer />
      </body>
    </html>
    </AuthProvider>
  );
}
