import { renderRobotsTxt } from '../lib/siteFiles';

// Built once, like the static file this route replaces.
export const dynamic = 'force-static';

/** GET /robots.txt, generated from the site origin. */
export function GET(): Response {
  return new Response(renderRobotsTxt(), {
    headers: { 'content-type': 'text/plain; charset=utf-8' },
  });
}
