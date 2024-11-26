interface Config {
  siteUrl: string;
  handlerUrl: string;
}

const config: Config = {
  siteUrl: process.env.NEXT_PUBLIC_DOMAIN_NAME || '',
  handlerUrl: process.env.NEXT_PUBLIC_HANDLER_URL || '',
};

export default config;
