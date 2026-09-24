/** Resolve the deployment's API endpoint without exposing the private URL. */
export function resolveHandlerUrl({ isBrowser, publicHandlerUrl, serverHandlerUrl }) {
  if (isBrowser) return publicHandlerUrl || '/api';
  if (serverHandlerUrl) return serverHandlerUrl;
  if (publicHandlerUrl && /^https?:\/\//i.test(publicHandlerUrl)) return publicHandlerUrl;
  return 'http://127.0.0.1:8081';
}

const isBrowser = typeof window !== 'undefined';
const config = {
  siteUrl: isBrowser
    ? process.env.NEXT_PUBLIC_DOMAIN_NAME
    : process.env.DOMAIN_NAME || process.env.NEXT_PUBLIC_DOMAIN_NAME,
  handlerUrl: resolveHandlerUrl({
    isBrowser,
    publicHandlerUrl: process.env.NEXT_PUBLIC_HANDLER_URL,
    serverHandlerUrl: process.env.HANDLER_URL,
  }),
};

export default config;
