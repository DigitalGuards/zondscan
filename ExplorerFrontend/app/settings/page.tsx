import { pageTitle, canonicalUrl } from '../lib/site';
import type { Metadata } from 'next';
import SettingsClient from './settings-client';

export const metadata: Metadata = {
  title: pageTitle('Site Settings'),
  description: 'Choose your appearance, address display, date format, and transaction preferences.',
  alternates: { canonical: canonicalUrl('/settings') },
};

export default function SettingsPage() {
  return <SettingsClient />;
}
