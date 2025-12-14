/** @type {import('next').NextConfig} */
const nextConfig = {
  distDir: 'build',
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
    '@mui/material': {
      transform: '@mui/material/{{member}}',
    },
    '@mui/icons-material': {
      transform: '@mui/icons-material/{{member}}',
    },
    '@heroicons/react/24/outline': {
      transform: '@heroicons/react/24/outline/{{member}}',
    },
    '@heroicons/react/20/solid': {
      transform: '@heroicons/react/20/solid/{{member}}',
    },
  },
  // Enable experimental optimizations
  experimental: {
    optimizePackageImports: ['@mui/material', '@mui/icons-material', '@visx/axis', '@visx/shape', '@visx/scale', '@visx/group'],
  },
}

module.exports = nextConfig
