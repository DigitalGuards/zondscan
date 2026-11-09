import { renderSecurityTxt } from '../../lib/siteFiles';

// Built once, like the static file this route replaces.
export const dynamic = 'force-static';

/** GET /.well-known/security.txt (RFC 9116), generated from the site origin. */
export function GET(): Response {
  return new Response(renderSecurityTxt(), {
    headers: { 'content-type': 'text/plain; charset=utf-8' },
  });
}
