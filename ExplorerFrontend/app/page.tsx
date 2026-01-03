import HomeClient from './home-client';
import { metadata } from './layout';


export default function Home(): JSX.Element {
  return <HomeClient pageTitle={metadata.title as string} />;
}
