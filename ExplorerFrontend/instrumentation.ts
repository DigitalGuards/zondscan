export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    const { validateDeployment } = await import('./app/lib/deployment-startup');
    await validateDeployment();
  }
}
