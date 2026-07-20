'use client';

import React, { useState } from 'react';
import { smallestUnitToDecimal, decimalToSmallestUnit } from '../lib/helpers';

function Converter(): JSX.Element {
  const [quanta, setQuanta] = useState("");
  const [shor, setShor] = useState("");
  const [error, setError] = useState("");

  const handleChangeShors = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const value = e.target.value;
    if (value === '') {
      setError('');
      setShor('');
      setQuanta('');
      return;
    }
    if (!/^\d+$/.test(value)) {
      setError("Invalid Input: Enter a whole number (Shor is indivisible)");
    } else {
      setError('');
      setShor(value);
      setQuanta(smallestUnitToDecimal(value, 18));
    }
  };

  const handleChangeQuanta = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const value = e.target.value;
    if (value === '') {
      setError('');
      setShor('');
      setQuanta('');
      return;
    }
    if (!/^\d*\.?\d*$/.test(value) || value === '.') {
      setError("Invalid Input: Enter a valid number");
    } else {
      setError('');
      setQuanta(value);
      setShor(decimalToSmallestUnit(value, 18));
    }
  };

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8">
      <div className="flex flex-col items-center justify-center">
        <h2 className="section-title mb-8">Unit Converter</h2>
        <div className="w-full max-w-md card p-8">
          <div className="space-y-6">
            {/* Quanta Input */}
            <div>
              <label htmlFor="quanta-input" className="block text-sm font-medium text-text-secondary mb-2">Quanta</label>
              <div className="relative">
                <input
                  id="quanta-input"
                  type="text"
                  value={quanta}
                  onChange={handleChangeQuanta}
                  placeholder="Enter amount in Quanta"
                  className="w-full px-4 py-3 bg-background text-text-primary rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300"
                />
                <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                  <span className="text-text-secondary">Quanta</span>
                </div>
              </div>
            </div>

            {/* Conversion Arrow */}
            <div className="flex justify-center">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
              </svg>
            </div>

            {/* Shor Input */}
            <div>
              <label htmlFor="shor-input" className="block text-sm font-medium text-text-secondary mb-2">Shor</label>
              <div className="relative">
                <input
                  id="shor-input"
                  type="text"
                  value={shor}
                  onChange={handleChangeShors}
                  placeholder="Enter amount in Shor"
                  className="w-full px-4 py-3 bg-background text-text-primary rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300"
                />
                <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                  <span className="text-text-secondary">Shor</span>
                </div>
              </div>
            </div>

            {/* Error Message */}
            {error && (
              <div className="p-4 bg-background rounded-lg border border-red-500/50">
                <div className="flex items-center text-error">
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  {error}
                </div>
              </div>
            )}

            {/* Info Box */}
            <div className="mt-6 p-4 bg-background rounded-lg border border-border">
              <p className="text-sm text-text-secondary">
                1 Quanta = 1,000,000,000,000,000,000 Shor (10^18)
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default Converter;
