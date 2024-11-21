import React, { useState } from 'react';
import axios from 'axios';
import config from '../../config';
import {toFixed} from '../lib/helpers.js';

function BalanceCheckTool() {
    const [address, setAddress] = useState("");
    const [blockNrOrHash, setBlockNrOrHash] = useState(null);
    const [balance, setBalance] = useState(null);
    // const [snapshotDate, setSnapshotDate] = useState(""); // Uncomment when needed

    // Request this from Kaushal on PoS: https://www.npmjs.com/package/ethereum-block-by-date
    const handleSubmit = async (event) => {
        event.preventDefault();

        const formData = new URLSearchParams();
        formData.append('address', address.replace(/\s/g, ''));
        // formData.append('snapshotDate', snapshotDate); // Uncomment when needed

        try {
            const response = await axios.post(`${config.handlerUrl}/getBalance`, formData, {
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded'
                }
            });

            console.log(response);

            if (response.data.balance != "header not found"){
                console.log(response.data)
                setBalance(toFixed(response.data.balance) + " QRL"); // Update the balance state
            } else{
                setBalance("Header not found (Not found on the blockchain)");
            }

        } catch (error) {
            console.error('Error fetching balance:', error);
            setBalance(null);
        }
    };

    return (
        <div className="flex flex-col items-center justify-center">
            <h2 className="text-lg mb-4">Account Balance Checker</h2>
            <form 
                className="flex flex-col items-center" 
                onSubmit={handleSubmit}
            >
                <input 
                    className="mb-3 p-2 border border-gray-300 rounded"
                    type="text" 
                    value={address} 
                    onChange={(e) => setAddress(e.target.value)} 
                    placeholder="Address"
                    required 
                />
                {/* Uncomment when date filter is needed
                <input 
                    className="mb-3 p-2 border border-gray-300 rounded"
                    type="date" 
                    value={snapshotDate} 
                    onChange={(e) => setSnapshotDate(e.target.value)} 
                    placeholder="Snapshot Date"
                />
                */}
                {balance !== null && (
                    <div className="mb-3 text-sm font-semibold">
                        Balance: {balance}
                    </div>
                )}
                <button 
                    type="submit" 
                    className="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
                >
                    Check Balance
                </button>
            </form>
        </div>
    );
}

export default BalanceCheckTool;
