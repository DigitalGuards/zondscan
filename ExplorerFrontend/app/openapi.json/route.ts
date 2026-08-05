import { NextResponse } from 'next/server';

import { openApiSpec } from '../lib/openapi';

/**
 * GET /openapi.json
 *
 * Serves the hand-written OpenAPI 3.1 document for the ZondScan public API.
 * Open CORS so external tooling (Swagger UI, code generators, agents) can
 * load it from any origin; cached for an hour since the spec only changes
 * on deploy.
 */
export function GET(): NextResponse {
  return NextResponse.json(openApiSpec, {
    headers: {
      'content-type': 'application/json',
      'access-control-allow-origin': '*',
      'cache-control': 'public, max-age=3600',
    },
  });
}
