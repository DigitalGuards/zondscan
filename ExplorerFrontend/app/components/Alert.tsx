'use client'

import { useState } from 'react'

interface AlertProps {
  variant?: 'info' | 'success' | 'warning' | 'error'
  children: React.ReactNode
  dismissible?: boolean
  onDismiss?: () => void
}

const VARIANT_STYLES = {
  info: {
    container: 'bg-blue-900/20 border-blue-500/30 text-blue-300',
    icon: 'text-blue-400',
  },
  success: {
    container: 'bg-green-900/20 border-green-500/30 text-green-300',
    icon: 'text-green-400',
  },
  warning: {
    container: 'bg-yellow-900/20 border-yellow-500/30 text-yellow-300',
    icon: 'text-yellow-400',
  },
  error: {
    container: 'bg-red-900/20 border-red-500/30 text-red-300',
    icon: 'text-red-400',
  },
} as const

const ICONS = {
  info: (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
      <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
    </svg>
  ),
  success: (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
    </svg>
  ),
  warning: (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
    </svg>
  ),
  error: (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
    </svg>
  ),
}

export default function Alert({ variant = 'info', children, dismissible = false, onDismiss }: AlertProps): JSX.Element | null {
  const [dismissed, setDismissed] = useState(false)

  if (dismissed) return null

  const styles = VARIANT_STYLES[variant]

  const handleDismiss = (): void => {
    setDismissed(true)
    onDismiss?.()
  }

  return (
    <div role="alert" className={`flex items-start gap-3 px-4 py-3 rounded-xl border ${styles.container}`}>
      <div className={`flex-shrink-0 mt-0.5 ${styles.icon}`}>
        {ICONS[variant]}
      </div>
      <div className="flex-1 text-sm">{children}</div>
      {dismissible && (
        <button
          onClick={handleDismiss}
          className="flex-shrink-0 p-1 rounded-md hover:bg-white/10 transition-colors"
          aria-label="Dismiss alert"
        >
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
            <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
          </svg>
        </button>
      )}
    </div>
  )
}
