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
    ];
  },
}

module.exports = nextConfig
