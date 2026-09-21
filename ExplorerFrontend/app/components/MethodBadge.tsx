import React from 'react'

/* Follows the shared badge recipe (see Badge.tsx). */
const colors: Record<string, string> = {
  GET: 'bg-info/10 text-info border-info/25',
  POST: 'bg-success/10 text-success border-success/25',
  PUT: 'bg-warning/10 text-warning border-warning/25',
  DELETE: 'bg-error/10 text-error border-error/25',
}

export default function MethodBadge({ method }: { method: string }) {
  return (
    <span
      title={`HTTP ${method} request`}
      className={`inline-block px-2 py-0.5 text-xs font-mono font-bold rounded-md border ${colors[method] || 'bg-surface-2 text-text-secondary border-border'}`}
    >
      {method}
    </span>
  )
}
