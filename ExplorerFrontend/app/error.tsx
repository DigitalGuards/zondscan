'use client'

import { useEffect } from 'react'

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error(error)
  }, [error])

  return (
    <div className="min-h-[60vh] flex items-center justify-center p-4">
      <div className="text-center max-w-md">
        <div className="mb-6">
          <svg className="mx-auto h-16 w-16 text-red-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
          </svg>
        </div>
        <h2 className="text-xl font-semibold text-white mb-2">Something went wrong</h2>
        <p className="text-gray-400 mb-6">An unexpected error occurred while loading this page.</p>
        <div className="flex gap-3 justify-center">
          <button onClick={reset} className="px-4 py-2 text-sm font-medium rounded-lg bg-[#ffa729] text-black hover:bg-[#ffb952] transition-colors">
            Try again
          </button>
          <a href="/" className="px-4 py-2 text-sm font-medium rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] hover:text-white transition-colors">
            Go home
          </a>
        </div>
      </div>
    </div>
  )
}
