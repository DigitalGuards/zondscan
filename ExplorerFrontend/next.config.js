/** @type {import('next').NextConfig} */
const nextConfig = {
  distDir: 'build',
  output: 'standalone',
  turbopack: {},
  env: {
    HANDLER_URL: process.env.HANDLER_URL,
    DOMAIN_NAME: process.env.DOMAIN_NAME,
  },
  // Phase 3a: allow next/image to render off-chain NFT metadata images.
  // The syncer's metadata fetcher resolves every NFT image URL through
  // its configured IPFS gateway (default https://qrlwallet.com/api/ipfs/),
  // so the primary allowed origins are the wallet backend's IPFS proxy
  // (prod + dev) plus a small fallback list of common public gateways
  // for the rare contract whose metadata image points outside the proxy.
  images: {
    remotePatterns: [
      { protocol: 'https', hostname: 'qrlwallet.com', pathname: '/api/ipfs/**' },
      { protocol: 'https', hostname: 'dev.qrlwallet.com', pathname: '/api/ipfs/**' },
      { protocol: 'https', hostname: 'ipfs.io', pathname: '/ipfs/**' },
      { protocol: 'https', hostname: 'cloudflare-ipfs.com', pathname: '/ipfs/**' },
      { protocol: 'https', hostname: 'nftstorage.link', pathname: '/ipfs/**' },
    ],
  },
  // Optimize bundle size by transforming barrel imports
  modularizeImports: {
    '@heroicons/react/24/outline': {
      transform: '@heroicons/react/24/outline/{{member}}',
    },
    '@heroicons/react/20/solid': {
      transform: '@heroicons/react/20/solid/{{member}}',
    },
  },
  // Enable experimental optimizations
  experimental: {
    optimizePackageImports: ['@visx/axis', '@visx/shape', '@visx/scale', '@visx/group'],
  },
  // Baseline security headers applied to every route. A Content-Security-Policy
  // is intentionally NOT set here: the app loads TradingView from
  // s3.tradingview.com, Cloudflare Turnstile, and inline schema.org JSON-LD
  // scripts, so a wrong CSP would break the site. CSP is deferred to a
  // dedicated pass that can build and verify the allowlist in isolation.
  async headers() {
    return [
      {
        source: '/:path*',
        headers: [
          { key: 'X-Frame-Options', value: 'SAMEORIGIN' },
          { key: 'X-Content-Type-Options', value: 'nosniff' },
          { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
          { key: 'Strict-Transport-Security', value: 'max-age=63072000; includeSubDomains; preload' },
          { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
        ],
      },
    ];
  },
  // Proxy /api/* requests to the backend API server
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.HANDLER_URL || 'http://127.0.0.1:8081'}/:path*`,
      },
      {
        source: '/dapp-example',
        destination: '/dapp-example/index.html',
      },
      {
        source: '/dapp-example/',
        destination: '/dapp-example/index.html',
      },
    ];
  },
}

module.exports = nextConfig
