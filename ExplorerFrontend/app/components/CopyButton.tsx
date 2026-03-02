'use client'

import { useState } from 'react'
import type { MouseEvent } from 'react'

interface CopyButtonProps {
  value: string
  label?: string
  size?: 'sm' | 'md'
  stopPropagation?: boolean
}

// Usage examples:
// <CopyButton value={address} label="Copy address" />
// <CopyButton value={hash} label="Copy hash" stopPropagation />
// <CopyButton value={hash} label="Copy hash" size="sm" stopPropagation />

export default function CopyButton({
  value,
  label = 'Copy to clipboard',
  size = 'md',
  stopPropagation = false,
}: CopyButtonProps): JSX.Element {
  const [copySuccess, setCopySuccess] = useState(false)

  const copyToClipboard = (e: MouseEvent<HTMLButtonElement>): void => {
    if (stopPropagation) {
      e.stopPropagation()
    }
    navigator.clipboard
      .writeText(value)
      .then(() => {
        setCopySuccess(true)
        setTimeout(() => setCopySuccess(false), 2000)
      })
      .catch(err => {
        console.error('Failed to copy text: ', err)
      })
  }

  const copyIcon = copySuccess ? (
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M5 13l4 4L19 7"
    />
  ) : (
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
    />
  )

  if (size === 'sm') {
    return (
      <button
        type="button"
        onClick={copyToClipboard}
        aria-label={label}
        className="inline-flex items-center p-1 rounded-md
                  bg-card-gradient border border-border hover:border-accent
                  transition-all duration-300 group"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-3 w-3 text-accent"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          {copyIcon}
        </svg>
        <span aria-live="polite" className="sr-only">
          {copySuccess ? 'Copied!' : ''}
        </span>
      </button>
    )
  }

  return (
    <div className="inline-block">
      <button
        type="button"
        onClick={copyToClipboard}
        aria-label={label}
        className="inline-flex items-center px-3 py-1.5 rounded-lg
                  bg-card-gradient border border-border hover:border-accent
                  transition-all duration-300 group"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-4 w-4 mr-1.5 text-accent"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          aria-hidden="true"
        >
          {copyIcon}
        </svg>
        <span
          aria-live="polite"
          className="text-sm text-gray-300 group-hover:text-accent transition-colors"
        >
          {copySuccess ? 'Copied!' : 'Copy'}
        </span>
      </button>
    </div>
  )
}
