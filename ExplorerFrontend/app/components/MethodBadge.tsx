import React from 'react'

const colors: Record<string, string> = {
  GET: 'bg-green-900/50 text-green-400 border-green-700',
  POST: 'bg-blue-900/50 text-blue-400 border-blue-700',
}

export default function MethodBadge({ method }: { method: string }) {
  return (
    <span className={`inline-block px-2 py-0.5 text-xs font-mono font-bold rounded border ${colors[method] || 'bg-gray-700 text-gray-300 border-gray-600'}`}>
      {method}
    </span>
  )
}
