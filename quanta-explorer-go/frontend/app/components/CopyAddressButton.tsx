"use client";

import React, { useState } from "react";

interface Props {
    address: string;
}

export default function CopyAddressButton({ address }: Props) {
    const [copySuccess, setCopySuccess] = useState('');

    const copyToClipboard = () => {
        navigator.clipboard.writeText(address)
            .then(() => {
                setCopySuccess('Copied!');
                setTimeout(() => setCopySuccess(''), 2000); // Remove the "Copied!" message after 2 seconds
            })
            .catch(err => {
                console.error('Failed to copy text: ', err);
            });
    };

    return (
        <div className="inline-block">
            <button className="px-3 py-1 bg-blue-500 text-white rounded-md" onClick={copyToClipboard}>Copy Address</button>
            {copySuccess && <span className="ml-2 text-green-500">{copySuccess}</span>}
        </div>
    );
}
