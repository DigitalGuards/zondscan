import React, { useState } from 'react';
import axios from 'axios';
import config from '../../config';
import {toFixed} from '../lib/helpers.js';

function BalanceCheckTool() {
    const [address, setAddress] = useState("");
    const [blockNrOrHash, setBlockNrOrHash] = useState(null);
    const [balance, setBalance] = useState(null);
    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState(null);

    const handleSubmit = async (event) => {
        event.preventDefault();
        setIsLoading(true);
        setError(null);

        const formData = new URLSearchParams();
        formData.append('address', address.replace(/\s/g, ''));

        try {
            const response = await axios.post(`${config.handlerUrl}/getBalance`, formData, {
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded'
                }
            });

            if (response.data.balance != "header not found") {
                setBalance(toFixed(response.data.balance) + " QRL");
                setError(null);
            } else {
                setBalance(null);
                setError("Address not found on the blockchain");
            }
        } catch (error) {
            console.error('Error fetching balance:', error);
            setBalance(null);
            setError("Failed to fetch balance. Please try again.");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="max-w-[1200px] mx-auto p-8">
            <div className="flex flex-col items-center justify-center">
                <h2 className="text-2xl font-bold mb-8 text-[#ffa729]">Account Balance Checker</h2>
                <div className="w-full max-w-md bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] p-8 rounded-lg border border-[#3d3d3d] shadow-xl">
                    <form 
                        className="flex flex-col items-center space-y-6" 
                        onSubmit={handleSubmit}
                    >
                        <div className="relative w-full">
                            <input 
                                className="w-full px-4 py-3 bg-[#1a1b1e] text-white rounded-lg border border-[#3d3d3d] focus:outline-none focus:border-[#ffa729] transition-all duration-300 pl-10"
                                type="text" 
                                value={address} 
                                onChange={(e) => setAddress(e.target.value)} 
                                placeholder="Enter QRL address"
                                required 
                            />
                            <svg
                                className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-[#ffa729]"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                                />
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z"
                                />
                            </svg>
                        </div>

                        {balance !== null && !error && (
                            <div className="w-full p-4 bg-[#1a1b1e] rounded-lg border border-[#3d3d3d]">
                                <div className="text-sm text-gray-400">Balance</div>
                                <div className="text-xl font-bold text-[#ffa729]">{balance}</div>
                            </div>
                        )}

                        {error && (
                            <div className="w-full p-4 bg-[#1a1b1e] rounded-lg border border-red-500/50">
                                <div className="text-sm text-red-400">{error}</div>
                            </div>
                        )}

                        <button 
                            type="submit" 
                            disabled={isLoading}
                            className="w-full px-6 py-3 bg-gradient-to-r from-[#ffa729] to-[#ffb954] text-white font-bold rounded-lg hover:from-[#ffb954] hover:to-[#ffa729] transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg"
                        >
                            {isLoading ? (
                                <div className="flex items-center justify-center">
                                    <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    Checking...
                                </div>
                            ) : (
                                'Check Balance'
                            )}
                        </button>
                    </form>
                </div>
            </div>
        </div>
    );
}

export default BalanceCheckTool;
