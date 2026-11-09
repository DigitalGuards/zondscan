import { NextResponse } from 'next/server';

import { renderOpenApiSpec } from '../lib/openapi';
import { SITE_NAME, SITE_ORIGIN } from '../lib/site';

/**
 * GET /openapi.json
 *
 * Serves the hand-written OpenAPI 3.1 document for the public API, with the
 * configured site origin and name substituted. Open CORS so external tooling
 * (Swagger UI, code generators, agents) can load it from any origin; cached
 * for an hour since the spec only changes on deploy.
 */
export function GET(): NextResponse {
  return NextResponse.json(renderOpenApiSpec({ origin: SITE_ORIGIN, name: SITE_NAME }), {
    headers: {
      'content-type': 'application/json',
      'access-control-allow-origin': '*',
      'cache-control': 'public, max-age=3600',
    },
  });
}
