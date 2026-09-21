const defaultServerHandlerUrl = 'http://127.0.0.1:8080';

/**
 * Keep browser requests on the explorer origin through `/api`. Server
 * components use the private backend URL supplied when the container starts.
 *
 * @param {{ isBrowser: boolean, publicHandlerUrl?: string, serverHandlerUrl?: string }} options
 * @returns {string}
 */
export function resolveHandlerUrl({ isBrowser, publicHandlerUrl, serverHandlerUrl }) {
  if (isBrowser) {
    return publicHandlerUrl || '/api';
  }
  if (serverHandlerUrl) {
    return serverHandlerUrl;
  }
  if (publicHandlerUrl && /^https?:\/\//i.test(publicHandlerUrl)) {
    return publicHandlerUrl;
  }
  return defaultServerHandlerUrl;
}

const isBrowser = typeof window !== 'undefined';
const publicHandlerUrl = process.env.NEXT_PUBLIC_HANDLER_URL;

const config = {
  siteUrl: isBrowser
    ? process.env.NEXT_PUBLIC_DOMAIN_NAME
    : process.env.DOMAIN_NAME || process.env.NEXT_PUBLIC_DOMAIN_NAME,
  handlerUrl: resolveHandlerUrl({
    isBrowser,
    publicHandlerUrl,
    serverHandlerUrl: process.env.HANDLER_URL,
  }),
};

export default config;
