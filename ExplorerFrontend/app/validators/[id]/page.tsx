import { Metadata } from 'next';
import { use } from 'react';
import ValidatorDetailClient from './validator-detail-client';

export const metadata: Metadata = {
  title: 'Validator Details | QRL Zond Explorer',
  description: 'View detailed information about a validator on the QRL Zond network',
};

export default function ValidatorDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <ValidatorDetailClient id={id} />;
}
