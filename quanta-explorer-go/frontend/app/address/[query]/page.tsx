import React from "react";
import axios from "axios";
import config from '../../../config';
import { STAKING_QUANTA } from '../../lib/constants'
import {toFixed} from '../../lib/helpers.js';
import { epochToISO } from '../../lib/helpers';
import { headers } from "next/headers";
import CopyAddressButton from "../../components/CopyAddressButton";
import TanStackTable from "../../components/TanStackTable";

const getData = async (url: string | URL) => {
    try {
        const path = new URL(url).pathname;

        // Check for favicon and return early
        if (path.endsWith('favicon.ico')) {
            return null;
        }

        if (!path.startsWith('/address/')) {
            console.error(`Invalid path: ${path}`);
            return null;
        }
        console.log(`${config.handlerUrl}/address/aggregate${path.replace('/address/', '/')}`)
        const response = await axios.get(`${config.handlerUrl}/address/aggregate${path.replace('/address/', '/')}`);
        return response.data;
    } catch (error) {
        console.error(`Error fetching data`);
        return null;
    }
};

export default async function Address() {
    const headersList = headers();
    const header_url = headersList.get('x-url') || "";
    const addressData = await getData(header_url);

    const {
        balance,
    } = addressData["address"];

    console.log(addressData["internal_transactions_by_address"]);

    const {
        rank
    } = addressData;

    let firstSeen = 0;
    let lastSeen = 0;
    if (addressData["transactions_by_address"] && Array.isArray(addressData["transactions_by_address"]) && addressData["transactions_by_address"].length > 0) {
        const timestamps = addressData["transactions_by_address"].map(tx => tx.TimeStamp);
        firstSeen = Math.min(...timestamps);
        lastSeen = Math.max(...timestamps);
    }


    const addressSegmentParts = new URL(header_url).pathname.split('/');
    const addressSegment = addressSegmentParts.length > 0 ? addressSegmentParts.pop()! : "";

    let addressType = "";
    if (addressSegment.slice(0, 3) === "0x2") {
        addressType = "Dilithium Address - ";
    } else if (addressSegment.slice(0, 3) === "0x1") {
        addressType = "XMSS Address - ";
    } else if (addressData.response !== null) {
        addressType = "Contract -  ";
    }

    return (
        <>
            <div className="border-b mb-4">
                <div className="text-xl m-2 break-words items-center">
                    {addressType}{addressSegment}
                    <span className="ml-2 mr-6">
                        <CopyAddressButton address={addressSegment} />
                    </span>
                </div>
            </div><div className="border-b mb-4">
                <div className="grid grid-cols-2 gap-4">
                    <div className="m-2 p-4 border rounded">
                        <div className="text-lg font-bold mb-2">Overview</div>
                        <div className="text-sm text-gray-600 mb-2">QRL BALANCE: </div>
                        <div>{toFixed(balance)}</div>
                        <br/>
                        <div><p><small>Note: Paid fees are always paid by the sending party</small></p></div>
                    </div>

                    <div className="m-2 p-4 border rounded">
                        <div className="text-lg font-bold mb-2">Additional Information</div>
                        <div className="mb-2">
                            {(epochToISO(firstSeen) === "1970-01-01" && epochToISO(lastSeen) === "1970-01-01")
                                ? "No transactions were signed from this wallet yet."
                                : `This wallet's first activity was on ${epochToISO(firstSeen).split('T')[0]} and was last seen on ${epochToISO(lastSeen).split('T')[0]}. Current rank is #${rank}. This wallet is ${balance > STAKING_QUANTA ? '' : 'not'} qualified to stake on the QRL Blockchain! ${balance > STAKING_QUANTA ? 'Congratulations!' : ''}`}
                        </div>

                        <button className="text-blue-500 hover:underline text-sm">Learn More</button>
                    </div>
                </div>
            </div>

            <TanStackTable transactions={addressData["transactions_by_address"]} internalt={addressData["internal_transactions_by_address"]} />
        </>
    );
}