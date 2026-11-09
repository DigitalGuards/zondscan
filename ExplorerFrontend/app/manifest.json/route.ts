import { NextResponse } from 'next/server';

import { SITE_NAME } from '../lib/site';
import { renderManifest } from '../lib/siteFiles';

// Built once, like the static file this route replaces.
export const dynamic = 'force-static';

/** GET /manifest.json, generated from the site name. */
export function GET(): NextResponse {
  return NextResponse.json(renderManifest({ name: SITE_NAME }), {
    headers: { 'content-type': 'application/json' },
  });
}
