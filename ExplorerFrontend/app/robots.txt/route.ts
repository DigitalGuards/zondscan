import { renderRobotsTxt } from '../lib/siteFiles';

/** GET /robots.txt, generated from the site origin. */
export function GET(): Response {
  return new Response(renderRobotsTxt(), {
    headers: { 'content-type': 'text/plain; charset=utf-8' },
  });
}
