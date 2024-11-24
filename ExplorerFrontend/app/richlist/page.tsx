import axios from "axios";
import config from "../../config";
import RichlistClient from "./richlist-client";

export default async function RichlistPage() {
  const response = await axios.get(config.handlerUrl + "/richlist");
  return <RichlistClient richlist={response.data.richlist} />;
}
