import config from "../../config";
import RichlistClient from "./richlist-client";

export default async function RichlistPage() {
  const response = await fetch(config.handlerUrl + "/richlist", {
    cache: 'no-store'
  });
  const data = await response.json();
  return <RichlistClient richlist={data.richlist} />;
}
