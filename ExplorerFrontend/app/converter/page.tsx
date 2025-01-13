"use client";

import React, { useState } from 'react';
import { toFixed } from '../lib/helpers';

function Converter() {
    const [quanta, setQuanta] = useState("");
    const [shor, setShor] = useState("");
    const [error, setError] = useState("");

    const DECIMALS = 1e18; // QRL has 18 decimal places like ethereum

    const handleChangeShors = (e: React.ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value;
        if (isNaN(Number(value))) {
            setError("Invalid Input: Enter a number");
        } else {
            setError('');
            setQuanta(toFixed((Number(value) / DECIMALS)).toString());
            setShor(value);
        }
    };

    const handleChangeQuanta = (e: React.ChangeEvent<HTMLInputElement>) => {
        const value = e.target.value;
        if (isNaN(Number(value))) {
            setError("Invalid Input: Enter a number");
        } else {
            setError('');
            setShor(toFixed((Number(value) * DECIMALS)).toString());
            setQuanta(value);
        }
    };

    return (
        <div className="max-w-[1200px] mx-auto p-8">
            <div className="flex flex-col items-center justify-center">
                <h2 className="text-2xl font-bold mb-8 text-[#ffa729]">Unit Converter</h2>
                <div className="w-full max-w-md bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] p-8 rounded-lg border border-[#3d3d3d] shadow-xl">
                    <div className="space-y-6">
                        {/* Quanta Input */}
                        <div>
                            <label className="block text-sm font-medium text-gray-300 mb-2">Quanta (QRL)</label>
                            <div className="relative">
                                <input
                                    type="text"
                                    value={quanta}
                                    onChange={handleChangeQuanta}
                                    placeholder="Enter amount in Quanta"
                                    className="w-full px-4 py-3 bg-[#1a1b1e] text-white rounded-lg border border-[#3d3d3d] focus:outline-none focus:border-[#ffa729] transition-all duration-300"
                                />
                                <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                                    <span className="text-gray-400">QRL</span>
                                </div>
                            </div>
                        </div>

                        {/* Conversion Arrow */}
                        <div className="flex justify-center">
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                            </svg>
                        </div>

                        {/* Shor Input */}
                        <div>
                            <label className="block text-sm font-medium text-gray-300 mb-2">Shor</label>
                            <div className="relative">
                                <input
                                    type="text"
                                    value={shor}
                                    onChange={handleChangeShors}
                                    placeholder="Enter amount in Shor"
                                    className="w-full px-4 py-3 bg-[#1a1b1e] text-white rounded-lg border border-[#3d3d3d] focus:outline-none focus:border-[#ffa729] transition-all duration-300"
                                />
                                <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                                    <span className="text-gray-400">Shor</span>
                                </div>
                            </div>
                        </div>

                        {/* Error Message */}
                        {error && (
                            <div className="p-4 bg-[#1a1b1e] rounded-lg border border-red-500/50">
                                <div className="flex items-center text-red-400">
                                    <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                    {error}
                                </div>
                            </div>
                        )}

                        {/* Info Box */}
                        <div className="mt-6 p-4 bg-[#1a1b1e] rounded-lg border border-[#3d3d3d]">
                            <p className="text-sm text-gray-400">
                                1 QRL = 1,000,000,000,000,000,000 Shor (10^18)
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default Converter;
