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
}

module.exports = nextConfig
