'use client'

import dynamic from 'next/dynamic'
import { Suspense } from 'react'

const ContractsClient = dynamic(() => import('./contracts-client'), {
  ssr: false,
})

interface ContractsWrapperProps {
  initialData: any[];
  totalContracts: number;
}

export default function ContractsWrapper({ initialData, totalContracts }: ContractsWrapperProps): JSX.Element {
  return (
    <Suspense fallback={
      <div role="status" aria-label="Loading contracts" className="p-4 space-y-4">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="h-16 bg-gray-700/30 rounded animate-pulse" />
        ))}
      </div>
    }>
      <ContractsClient initialData={initialData} totalContracts={totalContracts} />
    </Suspense>
  )
}
