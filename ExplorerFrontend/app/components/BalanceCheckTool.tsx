'use client';

import { useState } from 'react';
import type { FormEvent, ChangeEvent } from 'react';
import axios, { AxiosError } from 'axios';
import config from '../../config';
import { toFixed } from '../lib/helpers';

interface BalanceResponse {
  balance: string;
}

export default function BalanceCheckTool(): JSX.Element {
    const [address, setAddress] = useState<string>('');
    const [balance, setBalance] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState<boolean>(false);
    const [error, setError] = useState<string | null>(null);

    const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
        event.preventDefault();
        setIsLoading(true);
        setError(null);

        const formData = new URLSearchParams();
        formData.append('address', address.replace(/\s/g, ''));

        try {
            const response = await axios.post<BalanceResponse>(
                `${config.handlerUrl}/getBalance`,
                formData,
                {
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded'
                    }
                }
            );

            if (response.data.balance !== "header not found") {
                setBalance(`${toFixed(response.data.balance)} QRL`);
                setError(null);
            } else {
                setBalance(null);
                setError("Address not found on the blockchain");
            }
        } catch (err) {
            console.error('Error fetching balance:', err);
            setBalance(null);
            setError(err instanceof AxiosError 
                ? err.response?.data?.message || "Failed to fetch balance. Please try again."
                : "Failed to fetch balance. Please try again."
            );
        } finally {
            setIsLoading(false);
        }
    };

    const handleAddressChange = (e: ChangeEvent<HTMLInputElement>): void => {
        setAddress(e.target.value);
    };

    return (
        <div className="max-w-[1200px] mx-auto p-8">
            <div className="flex flex-col items-center justify-center">
                <h2 className="text-2xl font-bold mb-8 text-accent">Account Balance Checker</h2>
                <div className="w-full max-w-md bg-card-gradient p-8 rounded-lg border border-border shadow-xl">
                    <form 
                        className="flex flex-col items-center space-y-6" 
                        onSubmit={handleSubmit}
                    >
                        <div className="relative w-full">
                            <input
                                className="w-full px-4 py-3 bg-background text-white rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300 pl-10"
                                type="text" 
                                value={address} 
                                onChange={handleAddressChange} 
                                placeholder="Enter QRL address"
                                required 
                            />
                            <svg
                                className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-accent"
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
                            <div className="w-full p-4 bg-background rounded-lg border border-border">
                                <div className="text-sm text-gray-400">Balance</div>
                                <div className="text-xl font-bold text-accent">{balance}</div>
                            </div>
                        )}

                        {error && (
                            <div className="w-full p-4 bg-background rounded-lg border border-red-500/50">
                                <div className="text-sm text-red-400">{error}</div>
                            </div>
                        )}

                        <button
                            type="submit"
                            disabled={isLoading}
                            className="w-full px-6 py-3 bg-gradient-to-r from-accent to-accent-hover text-white font-bold rounded-lg hover:from-accent-hover hover:to-accent transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg"
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
