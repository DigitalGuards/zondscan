"use client";

import { useState } from "react";

interface Props {
  address: string;
}

export default function CopyAddressButton({ address }: Props): JSX.Element {
  const [copySuccess, setCopySuccess] = useState('');

  const copyToClipboard = (): void => {
        navigator.clipboard.writeText(address)
            .then(() => {
                setCopySuccess('Copied!');
                setTimeout(() => setCopySuccess(''), 2000);
            })
            .catch(err => {
                console.error('Failed to copy text: ', err);
            });
    };

    return (
        <div className="inline-block">
            <button
                onClick={copyToClipboard}
                className="inline-flex items-center px-3 py-1.5 rounded-lg
                          bg-card-gradient border border-border hover:border-accent
                          transition-all duration-300 group"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-4 w-4 mr-1.5 text-accent"
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
                <span className="text-sm text-gray-300 group-hover:text-accent transition-colors">
                    {copySuccess ? 'Copied!' : 'Copy'}
                </span>
            </button>
        </div>
    );
}
