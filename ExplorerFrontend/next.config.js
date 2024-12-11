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
  }
}

module.exports = nextConfig
