"use client";

import React, { useState } from "react";

interface Props {
    hash: string;
    size?: "small" | "normal";
}

export default function CopyHashButton({ hash, size = "normal" }: Props) {
    const [copySuccess, setCopySuccess] = useState('');

    const copyToClipboard = (e: React.MouseEvent) => {
        e.stopPropagation(); // Prevent card click when copying
        navigator.clipboard.writeText(hash)
            .then(() => {
                setCopySuccess('Copied!');
                setTimeout(() => setCopySuccess(''), 2000);
            })
            .catch(err => {
                console.error('Failed to copy text: ', err);
            });
    };

    if (size === "small") {
        return (
            <button 
                onClick={copyToClipboard}
                className="inline-flex items-center p-1 rounded-md
                          bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                          border border-[#3d3d3d] hover:border-[#ffa729]
                          transition-all duration-300 group"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-3 w-3 text-[#ffa729]"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                >
                    {copySuccess ? (
                        <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M5 13l4 4L19 7"
                        />
                    ) : (
                        <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                        />
                    )}
                </svg>
            </button>
        );
    }

    return (
        <button 
            onClick={copyToClipboard}
            className="inline-flex items-center px-3 py-1.5 rounded-lg
                      bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                      border border-[#3d3d3d] hover:border-[#ffa729]
                      transition-all duration-300 group"
        >
            <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-4 w-4 mr-1.5 text-[#ffa729]"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
            >
                {copySuccess ? (
                    <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M5 13l4 4L19 7"
                    />
                ) : (
                    <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                    />
                )}
            </svg>
            <span className="text-sm text-gray-300 group-hover:text-[#ffa729] transition-colors">
                {copySuccess ? 'Copied!' : 'Copy'}
            </span>
        </button>
    );
}
