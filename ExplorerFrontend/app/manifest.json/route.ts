import { NextResponse } from 'next/server';

import { SITE_NAME, SITE_ORIGIN } from '../lib/site';
import { renderManifest } from '../lib/siteFiles';

/** GET /manifest.json, generated from the site name. */
export function GET(): NextResponse {
  return NextResponse.json(renderManifest({ origin: SITE_ORIGIN, name: SITE_NAME }), {
    headers: { 'content-type': 'application/json' },
  });
}
