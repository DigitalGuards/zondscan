import HomeClient from './home-client';
import { metadata } from './layout';


export default function Home() {
  return <HomeClient pageTitle={metadata.title as string} />;
}
