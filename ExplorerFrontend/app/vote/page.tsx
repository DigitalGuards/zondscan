import VoteClient from './vote-client';
import config from "../../config";

export default async function VotePage() {
  const response = await fetch(config.handlerUrl + "/richlist", {
    cache: 'no-store'
  });
  const data = await response.json();
  return <VoteClient richlist={data.richlist} />;
}
