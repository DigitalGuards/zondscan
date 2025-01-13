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

export default function ContractsWrapper({ initialData, totalContracts }: ContractsWrapperProps) {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ContractsClient initialData={initialData} totalContracts={totalContracts} />
    </Suspense>
  )
}
