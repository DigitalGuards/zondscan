/** @type {import('next').NextConfig} */
const nextConfig = {
  distDir: 'build',
  output: 'standalone',
  // Turbopack configuration (empty for now, may need buffer polyfill in future)
  turbopack: {},
  webpack: (config, { isServer }) => {
    if (!isServer) {
      config.resolve.fallback = {
        ...config.resolve.fallback,
        buffer: require.resolve('buffer/'),
      };
    }
    return config;
  },
  transpilePackages: ['buffer'],
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
