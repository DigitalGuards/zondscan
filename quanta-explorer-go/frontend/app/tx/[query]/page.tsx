"use client";

import JSONFormatter from 'json-formatter-js'
// import "json-formatter-js/dist//json-formatter.css";
import axios from 'axios';
import { usePathname } from 'next/navigation'
import React, { useState, useEffect, useRef } from 'react';
import { decodeBase64ToHexadecimal } from "../../lib/helpers"
import config from '../../../config';
import Link from 'next/link';

function base64ToHex(str: string) {
  const raw = atob(str);
  let result = '';
  for (let i = 0; i < raw.length; i++) {
    const hex = raw.charCodeAt(i).toString(16);
    result += (hex.length === 2 ? hex : '0' + hex);
  }
  return result.toLowerCase();
}

export default function Tx() {
  const [isOpen, setIsOpen] = useState(false);
  const [transaction, setTransaction] = useState<any[]>([])
  const [latestBlock, setLatestBlock] = useState(0);
  const [timeStamp, setTimeStamp] = useState(0);
  const [sizeKb, setSize] = useState(0);
  const [nonce, setNonce] = useState(0);
  const [txBlock, setTxBlock] = useState(0);
  const [from, setFrom] = useState("");
  const [tos, setTos] = useState("");
  const ref = useRef<HTMLDivElement | null>(null);
  const pathname = usePathname()

  const formatter = new JSONFormatter(transaction);


  useEffect(() => {
    axios.get(`${config.handlerUrl}${pathname}`)
      .then(response => {
        const newTransactions = response.data;

        setTransaction(newTransactions);
        setNonce(response.data.response.nonce);
        setSize(response.data.response.size / 1024);
        setTimeStamp(response.data.response.blockTimestamp);
        setTxBlock(response.data.response.blockNumber);
        setLatestBlock(response.data.latestBlock[0].result.number);
        setFrom("0x" + decodeBase64ToHexadecimal(response.data.response.from))
        setTos("0x" + decodeBase64ToHexadecimal(response.data.response.to));

        if (ref.current) {
          let data = response.data.response;

          delete data.ID

          data.from = "0x" + decodeBase64ToHexadecimal(data.from);
          data.to = "0x" + decodeBase64ToHexadecimal(data.to);
          data.txHash = "0x" + decodeBase64ToHexadecimal(data.txHash);
          data.pk = "0x" + decodeBase64ToHexadecimal(data.pk);
          data.signature = "0x" + decodeBase64ToHexadecimal(data.signature);

          const formatter = new JSONFormatter(data, 1, {
            theme: "white"
          });
          formatter.openAtDepth(0);


          ref.current.innerHTML = '';
          ref.current.appendChild(formatter.render());
        }
      })
      .catch(error => console.error('Error fetching transactions:', error));
  }, []);

  formatter.openAtDepth(2);

  // console.log(sizeKb);

  const toggleOpen = () => setIsOpen(!isOpen);

  return (
    <>
      <div>

        <main className="flex bg-gray-100">
          <div className="flex max-w-6xl mx-auto p-4">

            <div className="bg-blue-400 p-4 rounded-lg shadow-lg mb-4 w-full">
              <div className="break-words text-xl font-bold w-auto">
                Transaction Details
              </div>
            </div>

            <div className="flex bg-gray-200 p-4 rounded-lg shadow-lg mb-4 w-full">
              <div className='flex-container'>
                <div className='flex-container justify-evenly'>

                  <div className='flex items-center'>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12c0 1.268-.63 2.39-1.593 3.068a3.745 3.745 0 0 1-1.043 3.296 3.745 3.745 0 0 1-3.296 1.043A3.745 3.745 0 0 1 12 21c-1.268 0-2.39-.63-3.068-1.593a3.746 3.746 0 0 1-3.296-1.043 3.745 3.745 0 0 1-1.043-3.296A3.745 3.745 0 0 1 3 12c0-1.268.63-2.39 1.593-3.068a3.745 3.745 0 0 1 1.043-3.296 3.746 3.746 0 0 1 3.296-1.043A3.746 3.746 0 0 1 12 3c1.268 0 2.39.63 3.068 1.593a3.746 3.746 0 0 1 3.296 1.043 3.746 3.746 0 0 1 1.043 3.296A3.745 3.745 0 0 1 21 12Z" />
                    </svg>

                    Confirmations {txBlock === null ? 0 : latestBlock - txBlock} confirmations
                  </div>
                  <div className='flex items-center'>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                      <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
                    </svg>

                    Block {txBlock}
                  </div>

                  <div className='flex items-center'>

                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 14.25v2.25m3-4.5v4.5m3-6.75v6.75m3-9v9M6 20.25h12A2.25 2.25 0 0 0 20.25 18V6A2.25 2.25 0 0 0 18 3.75H6A2.25 2.25 0 0 0 3.75 6v12A2.25 2.25 0 0 0 6 20.25Z" />
                    </svg>

                    Nonce {nonce}
                    {/* <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z" />
                </svg> */}
                    {/* 
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18.75a60.07 60.07 0 0 1 15.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 0 1 3 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 0 0-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 0 1-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 0 0 3 15h-.75M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm3 0h.008v.008H18V10.5Zm-12 0h.008v.008H6V10.5Z" />
                </svg> */}

                  </div>
                  <div className='flex items-center'>

                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9.568 3H5.25A2.25 2.25 0 0 0 3 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 0 0 5.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 0 0 9.568 3Z" />
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 6h.008v.008H6V6Z" />
                    </svg>


                    Size {sizeKb} KB
                  </div>
                  <div className='flex items-center'>
                    <div className="ml-2 w-3 h-3 bg-green-500 rounded-full">
                    </div>
                    Status OK
                  </div>

                  <div className='flex items-center'>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5m-9-6h.008v.008H12v-.008ZM12 15h.008v.008H12V15Zm0 2.25h.008v.008H12v-.008ZM9.75 15h.008v.008H9.75V15Zm0 2.25h.008v.008H9.75v-.008ZM7.5 15h.008v.008H7.5V15Zm0 2.25h.008v.008H7.5v-.008Zm6.75-4.5h.008v.008h-.008v-.008Zm0 2.25h.008v.008h-.008V15Zm0 2.25h.008v.008h-.008v-.008Zm2.25-4.5h.008v.008H16.5v-.008Zm0 2.25h.008v.008H16.5V15Z" />
                    </svg>

                    {new Date(timeStamp * 1000).toLocaleString('en-GB')
                    }
                  </div>
                </div>
              </div>


            </div>

            <div className='break-words bg-blue-400 p-4 rounded-lg shadow-lg mb-4 w-full'>
              <span>from: <Link href={"/address/" + from} className='break-words'>{from}</Link></span>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9" />
              </svg>
              <span className='break-words w-full'>
                {tos}
              </span>
            </div>

            <button className="collapsible" onClick={toggleOpen}>
              Meta
            </button>
            <div style={{ display: isOpen ? 'block' : 'none' }}>
              <div ref={ref} />
            </div>
          </div>
        </main>
      </div>
    </>
  )
}