'use client';

import { useState, useEffect } from 'react';
import axios from 'axios';
import config from '../../config';
import { toFixed, formatAmount, epochsToDays } from '../lib/helpers';
import { getDilithiumAddressFromPK } from '@theqrl/wallet.js';

// Convert base64 to hex string
function base64ToHex(base64: string): string {
  return Buffer.from(base64, 'base64').toString('hex');
}

interface Validator {
  address: string;
  uptime: number;
  age: number;
  stakedAmount: string;
  isActive: boolean;
}

export default function ValidatorsWrapper() {
  const [validators, setValidators] = useState<Validator[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [totalStaked, setTotalStaked] = useState('0');

  useEffect(() => {
    async function fetchValidators() {
      try {
        const response = await axios.get(`${config.handlerUrl}/validators`);
        const validatorsData = response.data.validators || [];
        
        // Process validators to decode addresses
        const processedValidators = validatorsData.map((validator: any) => {
          const publicKey = Buffer.from(validator.address, 'base64');
          const dilithiumAddress = getDilithiumAddressFromPK(publicKey);
          return {
            ...validator,
            address: '0x' + Buffer.from(dilithiumAddress).toString('hex')
          };
        });
        
        setValidators(processedValidators);
        setTotalStaked(response.data.totalStaked || '0');
      } catch (err) {
        console.error('Error fetching validators:', err);
        setError('Failed to load validator information. Please try again later.');
      } finally {
        setLoading(false);
      }
    }

    fetchValidators();
    const interval = setInterval(fetchValidators, 60000);
    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return <ValidatorsLoadingSkeleton />;
  }

  if (error) {
    return (
      <div className="p-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6">
          <h2 className="text-red-500 font-semibold mb-2">Error</h2>
          <p className="text-gray-300">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 lg:p-6">
      <div className="mb-8">
        <h2 className="text-2xl font-bold text-[#ffa729] mb-4">Network Statistics</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <StatCard
            title="Total Validators"
            value={validators.length.toString()}
            icon={
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
              </svg>
            }
          />
          <StatCard
            title="Active Validators"
            value={validators.filter(v => v.isActive).length.toString()}
            icon={
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
          />
          <StatCard
            title="Total Staked"
            value={formatAmount(totalStaked)[0]}
            icon={
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
          />
        </div>
      </div>

      <div className="mb-8">
        <h2 className="text-2xl font-bold text-[#ffa729] mb-4">Validator List</h2>
        <div className="overflow-x-auto -mx-4 sm:mx-0">
          <div className="inline-block min-w-full align-middle">
            <div className="overflow-hidden border border-[#3d3d3d] rounded-xl">
              <table className="min-w-full divide-y divide-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]">
                <thead>
                  <tr className="bg-[#2d2d2d]/50">
                    <th scope="col" className="py-3.5 pl-4 pr-3 text-left text-sm font-semibold text-gray-400 sm:pl-6">Validator</th>
                    <th scope="col" className="px-3 py-3.5 text-left text-sm font-semibold text-gray-400">Status</th>
                    <th scope="col" className="hidden sm:table-cell px-3 py-3.5 text-right text-sm font-semibold text-gray-400">Age</th>
                    <th scope="col" className="hidden sm:table-cell px-3 py-3.5 text-right text-sm font-semibold text-gray-400">Uptime</th>
                    <th scope="col" className="px-3 py-3.5 text-right text-sm font-semibold text-gray-400 sm:pr-6">Staked</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#3d3d3d] bg-transparent">
                  {validators.map((validator, index) => (
                    <tr key={`table-${validator.address}-${index}`} className="hover:bg-[#2d2d2d]/30">
                      <td className="whitespace-nowrap py-4 pl-4 pr-3 text-sm sm:pl-6">
                        <a 
                          href={`/address/${validator.address}`}
                          className="text-[#ffa729] hover:underline font-mono text-xs sm:text-sm"
                        >
                          {window.innerWidth < 640 
                            ? `${validator.address.slice(0, 6)}...${validator.address.slice(-4)}`
                            : `${validator.address.slice(0, 10)}...${validator.address.slice(-8)}`}
                        </a>
                      </td>
                      <td className="whitespace-nowrap px-3 py-4 text-sm">
                        <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                          validator.isActive 
                            ? 'bg-green-100 text-green-800' 
                            : 'bg-red-100 text-red-800'
                        }`}>
                          {validator.isActive ? 'Active' : 'Inactive'}
                        </span>
                      </td>
                      <td className="hidden sm:table-cell whitespace-nowrap px-3 py-4 text-sm text-gray-300 text-right">
                        {epochsToDays(validator.age).toFixed(1)} days
                        <span className="text-gray-500 text-xs ml-1">({validator.age} epochs)</span>
                      </td>
                      <td className="hidden sm:table-cell whitespace-nowrap px-3 py-4 text-sm text-gray-300 text-right">{validator.uptime.toFixed(2)}%</td>
                      <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-300 text-right sm:pr-6 font-mono">
                        {formatAmount(validator.stakedAmount)[0]}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
        
        {/* Mobile-only expandable details */}
        <div className="sm:hidden mt-4 space-y-4">
          {validators.map((validator, index) => (
            <details key={`mobile-${validator.address}-${index}`} className="bg-[#2d2d2d]/30 rounded-lg">
              <summary className="px-4 py-3 cursor-pointer hover:bg-[#2d2d2d]/50 rounded-lg">
                <div className="flex items-center justify-between">
                  <a 
                    href={`/address/${validator.address}`}
                    className="text-[#ffa729] hover:underline font-mono text-sm"
                  >
                    {`${validator.address.slice(0, 6)}...${validator.address.slice(-4)}`}
                  </a>
                  <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                    validator.isActive 
                      ? 'bg-green-100 text-green-800' 
                      : 'bg-red-100 text-red-800'
                  }`}>
                    {validator.isActive ? 'Active' : 'Inactive'}
                  </span>
                </div>
              </summary>
              <div className="px-4 py-3 border-t border-[#3d3d3d] space-y-2">
                <div className="flex justify-between">
                  <span className="text-gray-400">Age:</span>
                  <span className="text-gray-300">
                    {epochsToDays(validator.age).toFixed(1)} days
                    <span className="text-gray-500 text-xs ml-1">({validator.age} epochs)</span>
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Uptime:</span>
                  <span className="text-gray-300">{validator.uptime.toFixed(2)}%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Staked:</span>
                  <span className="text-gray-300 font-mono">{formatAmount(validator.stakedAmount)[0]} QRL</span>
                </div>
              </div>
            </details>
          ))}
        </div>
      </div>
    </div>
  );
}

function StatCard({ title, value, icon }: { title: string; value: string; icon: React.ReactNode }) {
  return (
    <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4 sm:p-6">
      <div className="flex items-center space-x-4">
        <div className="flex-shrink-0">
          <div className="p-2 sm:p-3 bg-[#ffa729]/10 rounded-lg text-[#ffa729]">
            {icon}
          </div>
        </div>
        <div>
          <p className="text-xs sm:text-sm text-gray-400">{title}</p>
          <p className="text-lg sm:text-2xl font-bold text-[#ffa729]">{value}</p>
        </div>
      </div>
    </div>
  );
}

function ValidatorsLoadingSkeleton() {
  return (
    <div className="p-4 lg:p-6 animate-pulse">
      <div className="mb-8">
        <div className="h-8 w-48 bg-[#2d2d2d] rounded mb-4"></div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="bg-[#2d2d2d] rounded-xl h-24 sm:h-32"></div>
          ))}
        </div>
      </div>
      <div className="mb-8">
        <div className="h-8 w-48 bg-[#2d2d2d] rounded mb-4"></div>
        <div className="bg-[#2d2d2d] rounded-xl h-64 sm:h-96"></div>
      </div>
    </div>
  );
}
