import { NextRequest, NextResponse } from 'next/server';
import { resolveSearchPath } from '../lib/searchResolver';

/**
 * GET /search?q=<address | tx hash | block number>
 *
 * Server-side twin of the client SearchBar: resolves the query with the
 * same pure resolver and 302-redirects to the matching detail page.
 * Exists primarily so the schema.org WebSite SearchAction in the root
 * layout points at a real endpoint (Google sitelinks search box), but it
 * also makes bookmarkable search URLs work. Unresolvable queries fall
 * back to the homepage.
 */
export function GET(request: NextRequest): NextResponse {
  try {
    const query = request.nextUrl.searchParams.get('q') ?? '';
    const result = resolveSearchPath(query);
    let destination = 'path' in result ? result.path : '/';
    // Only allow same-origin relative paths; '//host' would redirect off-site.
    if (!destination.startsWith('/') || destination.startsWith('//')) {
      destination = '/';
    }
    return NextResponse.redirect(new URL(destination, request.nextUrl.origin), 302);
  } catch {
    return NextResponse.redirect(new URL('/', request.nextUrl.origin), 302);
  }
}
