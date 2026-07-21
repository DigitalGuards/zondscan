'use client';

import React, { useState } from 'react';
import { convertUnits, NATIVE_UNIT } from '../lib/helpers';
import type { ConvertibleUnit } from '../lib/helpers';

const UNITS: ConvertibleUnit[] = ['Quanta', 'Shor', 'Planck'];

const INPUT_CLASSES =
  'w-full px-4 py-3 bg-background text-text-primary rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300';

// The Quanta option label reuses NATIVE_UNIT so a unit rename stays one-line.
function unitLabel(unit: ConvertibleUnit): string {
  return unit === 'Quanta' ? NATIVE_UNIT : unit;
}

// Shown when the input is a valid number but has more fractional digits
// than the source unit can represent (convertUnits returns null).
const FRACTION_ERRORS: Record<ConvertibleUnit, string> = {
  Quanta: `${NATIVE_UNIT} supports at most 18 decimal places`,
  Shor: 'Shor supports at most 9 decimal places',
  Planck: 'Planck is indivisible',
};

// Selects only ever offer the three known units; fall back to Quanta so
// the value narrows to ConvertibleUnit without a type assertion.
function parseUnit(value: string): ConvertibleUnit {
  return value === 'Shor' || value === 'Planck' ? value : 'Quanta';
}

function Converter(): JSX.Element {
  const [amount, setAmount] = useState('');
  const [fromUnit, setFromUnit] = useState<ConvertibleUnit>('Quanta');
  const [toUnit, setToUnit] = useState<ConvertibleUnit>('Shor');

  // Keep the raw string in state and derive result + error every render:
  // keystrokes are never swallowed, invalid input just surfaces an error.
  const trimmed = amount.trim();
  const result = trimmed === '' ? '' : convertUnits(trimmed, fromUnit, toUnit);
  const isPlainNumber = /^(\d+\.?\d*|\.\d+)$/.test(trimmed);
  const error =
    trimmed === '' || result !== null
      ? ''
      : isPlainNumber
        ? FRACTION_ERRORS[fromUnit]
        : 'Invalid Input: Enter a valid number';

  const handleSwap = (): void => {
    setFromUnit(toUnit);
    setToUnit(fromUnit);
  };

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8">
      <div className="flex flex-col items-center justify-center">
        <h2 className="section-title mb-8">Unit Converter</h2>
        <div className="w-full max-w-md card p-8">
          <div className="space-y-6">
            {/* Amount Input */}
            <div>
              <label htmlFor="amount-input" className="block text-sm font-medium text-text-secondary mb-2">Amount</label>
              <input
                id="amount-input"
                type="text"
                inputMode="decimal"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
                placeholder={`Enter amount in ${unitLabel(fromUnit)}`}
                className={INPUT_CLASSES}
              />
            </div>

            {/* Unit Selects + Swap */}
            <div className="flex items-end gap-3">
              <div className="flex-1">
                <label htmlFor="from-unit" className="block text-sm font-medium text-text-secondary mb-2">From</label>
                <select
                  id="from-unit"
                  value={fromUnit}
                  onChange={(e) => setFromUnit(parseUnit(e.target.value))}
                  className={INPUT_CLASSES}
                >
                  {UNITS.map((unit) => (
                    <option key={unit} value={unit}>{unitLabel(unit)}</option>
                  ))}
                </select>
              </div>
              <button
                type="button"
                onClick={handleSwap}
                aria-label="Swap units"
                className="p-3 rounded-lg border border-border text-accent hover:border-accent hover:bg-background transition-all duration-300"
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                </svg>
              </button>
              <div className="flex-1">
                <label htmlFor="to-unit" className="block text-sm font-medium text-text-secondary mb-2">To</label>
                <select
                  id="to-unit"
                  value={toUnit}
                  onChange={(e) => setToUnit(parseUnit(e.target.value))}
                  className={INPUT_CLASSES}
                >
                  {UNITS.map((unit) => (
                    <option key={unit} value={unit}>{unitLabel(unit)}</option>
                  ))}
                </select>
              </div>
            </div>

            {/* Result */}
            <div>
              <label htmlFor="result-output" className="block text-sm font-medium text-text-secondary mb-2">Result</label>
              <div className="relative">
                <input
                  id="result-output"
                  type="text"
                  readOnly
                  value={result ?? ''}
                  placeholder="0"
                  className={`${INPUT_CLASSES} pr-20`}
                />
                <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none">
                  <span className="text-text-secondary">{unitLabel(toUnit)}</span>
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
                1 {NATIVE_UNIT} = 10^9 Shor = 10^18 Planck
              </p>
              <p className="text-sm text-text-secondary mt-1">
                1 Shor = 10^9 Planck
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default Converter;
