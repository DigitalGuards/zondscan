"use client"

import dynamic from 'next/dynamic';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';

interface ContractsWrapperProps {
  initialData: any[];
}

const ContractsClient = dynamic(() => import('./contracts-client'), {
  ssr: false,
  loading: () => (
    <Box className="p-4 text-center">
      <Typography>Loading contracts table...</Typography>
    </Box>
  ),
});

export default function ContractsWrapper({ initialData }: ContractsWrapperProps) {
  return <ContractsClient initialData={initialData} />;
}
