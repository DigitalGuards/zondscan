import * as React from 'react';
import axios from "axios";
import { formatNumber, formatNumberWithCommas } from "./lib/helpers";
import config from "../config.js"
import { toFixed } from "./lib/helpers.js"
import SearchBar from "./components/SearchBar"

export default async function Home() {

  const response = await axios.get(config.handlerUrl + "/overview");
  
  const data = {
    marketCapUSD: formatNumber(response.data.marketcap),
    walletCount: response.data.countwallets,
    volume: response.data.volume,
  };

  console.log(response.data.volume);

  const stats = [
    {
      data: data.walletCount,
      title: "Wallet count"
    },
    {
      data: toFixed(data.volume) + " QRL",
      title: "Daily Transactions Volume"
    },
  ]

  return (
    <>
    < SearchBar />
    <section className="py-14">
      <div className="max-w-screen-l mx-auto px-4 md:px-8">
        <div className="mt-12">
          <ul className="flex flex-col items-center justify-center gap-y-10 sm:flex-row sm:flex-wrap lg:divide-x">
            {
              stats.map((item, idx) => (
                <li key={idx} className="text-center px-12 md:px-16">
                  <h4 className="text-2xl font-semibold" style={{ color: "#FFA729" }}>{item.data}</h4>
                  <p className="mt-3 font-medium">{item.title}</p>
                </li>
              ))
            }
          </ul>
        </div>
      </div>
    </section>
    </>
  )
};
