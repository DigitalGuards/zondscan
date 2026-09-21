import type { Metadata } from 'next';
import SettingsClient from './settings-client';

export const metadata: Metadata = {
  title: 'Site Settings | ZondScan',
  description: 'Choose your appearance, address display, date format, and transaction preferences.',
  alternates: { canonical: 'https://zondscan.com/settings' },
};

export default function SettingsPage() {
  return <SettingsClient />;
}
