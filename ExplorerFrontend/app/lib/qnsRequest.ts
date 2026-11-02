import 'server-only';

/**
 * Optional server-only header that lets the explorer's own server calls skip
 * the backend's per-IP QNS limit. An unset token sends no header.
 */
export function qnsRequestHeaders(): Pick<RequestInit, 'headers'> {
  const token = process.env.QNS_INTERNAL_TOKEN;
  return token ? { headers: { 'X-QNS-Internal-Token': token } } : {};
}
