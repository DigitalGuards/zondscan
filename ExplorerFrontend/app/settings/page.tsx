import { SITE_NAME, siteUrl } from '../lib/site';
import type { Metadata } from 'next';
import SettingsClient from './settings-client';

export const metadata: Metadata = {
  title: `Site Settings | ${SITE_NAME}`,
  description: 'Choose your appearance, address display, date format, and transaction preferences.',
  alternates: { canonical: siteUrl('/settings') },
};

export default function SettingsPage() {
  return <SettingsClient />;
}
