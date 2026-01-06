'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import axios from 'axios';
import Link from 'next/link';
import config from '../../../config';
import { epochsToDays } from '../../lib/helpers';

// Format staked amount (beacon chain stores effective balance in Shor, 1 QRL = 10^9 Shor)
function formatValidatorBalance(amount: string): [string, string] {
  if (!amount || amount === '0') return ['0', 'QRL'];
  try {
    const value = BigInt(amount);
    const divisor = BigInt('1000000000'); // 10^9 (Shor to QRL)
    const qrlValue = Number(value / divisor);
    return [qrlValue.toLocaleString(undefined, { maximumFractionDigits: 0 }), 'QRL'];
  } catch {
    return ['0', 'QRL'];
  }
}

interface ValidatorDetail {
  index: string;
  publicKeyHex: string;
  withdrawalCredentialsHex: string;
  effectiveBalance: string;
  slashed: boolean;
  activationEligibilityEpoch: string;
  activationEpoch: string;
  exitEpoch: string;
  withdrawableEpoch: string;
  status: string;
  age: number;
  currentEpoch: string;
}

interface ValidatorDetailClientProps {
  id: string;
}

const FAR_FUTURE_EPOCH = '18446744073709551615';

export default function ValidatorDetailClient({ id }: ValidatorDetailClientProps) {
  const router = useRouter();
  const [validator, setValidator] = useState<ValidatorDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    async function fetchValidator() {
      try {
        setLoading(true);
        const response = await axios.get(`${config.handlerUrl}/validator/${id}`);
        setValidator(response.data);
        setError(null);
      } catch (err: any) {
        console.error('Error fetching validator:', err);
        setError(err.response?.data?.error || 'Failed to load validator details');
      } finally {
        setLoading(false);
      }
    }

    fetchValidator();
  }, [id]);

  const copyToClipboard = async (text: string, label: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(label);
      setTimeout(() => setCopied(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  const getStatusBadge = (status: string) => {
    const styles: Record<string, string> = {
      active: 'bg-green-900/30 text-green-400 border-green-800',
      pending: 'bg-yellow-900/30 text-yellow-400 border-yellow-800',
      exited: 'bg-gray-900/30 text-gray-400 border-gray-700',
      slashed: 'bg-red-900/30 text-red-400 border-red-800',
    };

    return (
      <span
        className={`inline-flex items-center px-3 py-1 rounded-full text-sm font-medium border ${
          styles[status] || styles.pending
        }`}
      >
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    );
  };

  const formatEpoch = (epoch: string): string => {
    if (epoch === FAR_FUTURE_EPOCH) return 'N/A';
    return parseInt(epoch).toLocaleString();
  };

  if (loading) {
    return (
      <div className="max-w-4xl mx-auto p-4 sm:p-6 lg:p-8">
        <div className="animate-pulse space-y-6">
          <div className="h-8 bg-gray-700 rounded w-1/3"></div>
          <div className="bg-[#2d2d2d] rounded-xl p-6 space-y-4">
            <div className="h-6 bg-gray-700 rounded w-2/3"></div>
            <div className="h-6 bg-gray-700 rounded w-1/2"></div>
            <div className="h-6 bg-gray-700 rounded w-3/4"></div>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl mx-auto p-4 sm:p-6 lg:p-8">
        <div className="bg-red-900/20 border border-red-800 rounded-xl p-6 text-center">
          <h2 className="text-xl font-semibold text-red-400 mb-2">Error</h2>
          <p className="text-gray-400">{error}</p>
          <button
            onClick={() => router.back()}
            className="mt-4 px-4 py-2 bg-[#2d2d2d] border border-[#3d3d3d] rounded-lg text-gray-300 hover:border-[#ffa729]"
          >
            Go Back
          </button>
        </div>
      </div>
    );
  }

  if (!validator) {
    return null;
  }

  const [amount, unit] = formatValidatorBalance(validator.effectiveBalance);

  return (
    <div className="max-w-4xl mx-auto p-4 sm:p-6 lg:p-8">
      {/* Back Button */}
      <Link
        href="/validators"
        className="inline-flex items-center text-gray-400 hover:text-[#ffa729] mb-6"
      >
        <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
        </svg>
        Back to Validators
      </Link>

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">
            Validator #{validator.index}
          </h1>
          <p className="text-gray-400 mt-1">
            Current Epoch: {parseInt(validator.currentEpoch).toLocaleString()}
          </p>
        </div>
        <div className="flex items-center gap-4">
          {getStatusBadge(validator.status)}
          {validator.slashed && (
            <span className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-red-900/50 text-red-300 border border-red-700">
              Slashed
            </span>
          )}
        </div>
      </div>

      {/* Key Info Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-sm font-medium text-gray-400 mb-1">Effective Balance</h3>
          <p className="text-xl font-semibold text-[#ffa729]">{amount} {unit}</p>
        </div>
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-sm font-medium text-gray-400 mb-1">Age</h3>
          <p className="text-xl font-semibold text-gray-200">
            {epochsToDays(validator.age).toFixed(1)} days
          </p>
          <p className="text-sm text-gray-500">{validator.age.toLocaleString()} epochs</p>
        </div>
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-sm font-medium text-gray-400 mb-1">Activation Epoch</h3>
          <p className="text-xl font-semibold text-green-400">
            {formatEpoch(validator.activationEpoch)}
          </p>
        </div>
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-sm font-medium text-gray-400 mb-1">Exit Epoch</h3>
          <p className={`text-xl font-semibold ${
            validator.exitEpoch === FAR_FUTURE_EPOCH ? 'text-gray-500' : 'text-red-400'
          }`}>
            {formatEpoch(validator.exitEpoch)}
          </p>
        </div>
      </div>

      {/* Details Section */}
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] overflow-hidden">
        <div className="p-4 border-b border-[#3d3d3d]">
          <h2 className="text-lg font-semibold text-[#ffa729]">Validator Details</h2>
        </div>
        <div className="divide-y divide-[#3d3d3d]">
          {/* Public Key */}
          <div className="p-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <span className="text-sm text-gray-400">Public Key</span>
              <div className="flex items-center gap-2">
                <code className="text-sm text-gray-300 font-mono break-all">
                  {validator.publicKeyHex}
                </code>
                <button
                  onClick={() => copyToClipboard(validator.publicKeyHex, 'publicKey')}
                  className="text-gray-400 hover:text-[#ffa729] p-1"
                  title="Copy to clipboard"
                >
                  {copied === 'publicKey' ? (
                    <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  ) : (
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                  )}
                </button>
              </div>
            </div>
          </div>

          {/* Withdrawal Credentials */}
          <div className="p-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
              <span className="text-sm text-gray-400">Withdrawal Credentials</span>
              <div className="flex items-center gap-2">
                <code className="text-sm text-gray-300 font-mono break-all">
                  {validator.withdrawalCredentialsHex}
                </code>
                <button
                  onClick={() => copyToClipboard(validator.withdrawalCredentialsHex, 'withdrawal')}
                  className="text-gray-400 hover:text-[#ffa729] p-1"
                  title="Copy to clipboard"
                >
                  {copied === 'withdrawal' ? (
                    <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                  ) : (
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                    </svg>
                  )}
                </button>
              </div>
            </div>
          </div>

          {/* Epoch Timeline */}
          <div className="p-4">
            <h3 className="text-sm font-medium text-gray-400 mb-4">Epoch Timeline</h3>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 rounded-full bg-blue-400"></div>
                  <span className="text-gray-300">Activation Eligibility</span>
                </div>
                <span className="text-gray-400 font-mono">
                  Epoch {formatEpoch(validator.activationEligibilityEpoch)}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="w-2 h-2 rounded-full bg-green-400"></div>
                  <span className="text-gray-300">Activation</span>
                </div>
                <span className="text-gray-400 font-mono">
                  Epoch {formatEpoch(validator.activationEpoch)}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full ${
                    validator.exitEpoch === FAR_FUTURE_EPOCH ? 'bg-gray-600' : 'bg-red-400'
                  }`}></div>
                  <span className="text-gray-300">Exit</span>
                </div>
                <span className="text-gray-400 font-mono">
                  {validator.exitEpoch === FAR_FUTURE_EPOCH ? 'Not scheduled' : `Epoch ${formatEpoch(validator.exitEpoch)}`}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className={`w-2 h-2 rounded-full ${
                    validator.withdrawableEpoch === FAR_FUTURE_EPOCH ? 'bg-gray-600' : 'bg-purple-400'
                  }`}></div>
                  <span className="text-gray-300">Withdrawable</span>
                </div>
                <span className="text-gray-400 font-mono">
                  {validator.withdrawableEpoch === FAR_FUTURE_EPOCH ? 'Not scheduled' : `Epoch ${formatEpoch(validator.withdrawableEpoch)}`}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
