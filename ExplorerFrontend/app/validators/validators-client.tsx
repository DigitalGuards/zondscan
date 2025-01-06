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
  const [currentPage, setCurrentPage] = useState(1);
  const [searchQuery, setSearchQuery] = useState('');
  const itemsPerPage = 10;

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

  // Filter validators based on search query
  const filteredValidators = validators.filter((validator: any) => {
    const searchLower = searchQuery.toLowerCase();
    return (
      validator.address.toLowerCase().includes(searchLower) ||
      validator.stakedAmount.toLowerCase().includes(searchLower)
    );
  });

  const totalPages = Math.ceil(filteredValidators.length / itemsPerPage);
  const startIndex = (currentPage - 1) * itemsPerPage;
  const endIndex = startIndex + itemsPerPage;
  const currentValidators = filteredValidators.slice(startIndex, endIndex);

  const goToPage = (page: number) => {
    setCurrentPage(page);
  };

  if (loading) {
    return <div className="p-4 text-gray-300">Loading validators...</div>;
  }

  if (error) {
    return <div className="p-4 text-red-500">Error: {error}</div>;
  }

  return (
    <div className="max-w-7xl mx-auto p-4 lg:p-6">
      {/* Statistics Section */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-lg font-semibold text-gray-400">Total Validators</h3>
          <p className="text-2xl text-[#ffa729]">{validators.length}</p>
        </div>
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-lg font-semibold text-gray-400">Active Validators</h3>
          <p className="text-2xl text-[#ffa729]">{validators.filter(v => v.isActive).length}</p>
        </div>
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4 sm:col-span-2 lg:col-span-1">
          <h3 className="text-lg font-semibold text-gray-400">Total Staked</h3>
          <p className="text-2xl text-[#ffa729]">{formatAmount(totalStaked)[0]} {formatAmount(totalStaked)[1]}</p>
        </div>
      </div>

      {/* Search Bar */}
      <div className="mb-6">
        <input
          type="text"
          placeholder="Search validator by address or staked amount..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full p-2 bg-[#2d2d2d] border border-[#3d3d3d] rounded-lg text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-[#ffa729] focus:border-transparent"
        />
      </div>

      {/* Validators List */}
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-[#3d3d3d]">
            <thead className="bg-[#2d2d2d]/50">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">Validator</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">Status</th>
                <th className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">Age</th>
                <th className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">Uptime</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">Staked</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#3d3d3d]">
              {currentValidators.map((validator, index) => (
                <tr key={index} className="hover:bg-[#2d2d2d]/30">
                  <td className="px-4 py-3 whitespace-nowrap text-sm">
                    <a href={`/address/${validator.address}`} className="text-[#ffa729] hover:underline font-mono">
                      {window.innerWidth < 640 
                        ? `${validator.address.slice(0, 8)}...${validator.address.slice(-6)}`
                        : validator.address}
                    </a>
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm">
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                      validator.isActive
                        ? 'bg-green-900/20 text-green-400'
                        : 'bg-red-900/20 text-red-400'
                    }`}>
                      {validator.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 whitespace-nowrap text-sm text-gray-300">
                    {epochsToDays(validator.age).toFixed(1)} days
                  </td>
                  <td className="hidden sm:table-cell px-4 py-3 whitespace-nowrap text-sm text-gray-300">
                    {validator.uptime.toFixed(2)}%
                  </td>
                  <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-300 font-mono">
                    {formatAmount(validator.stakedAmount)[0]} {formatAmount(validator.stakedAmount)[1]}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Mobile Expandable Details */}
        <div className="sm:hidden">
          {currentValidators.map((validator, index) => (
            <details key={`mobile-${index}`} className="border-t border-[#3d3d3d]">
              <summary className="px-4 py-3 cursor-pointer hover:bg-[#2d2d2d]/30">
                <div className="flex items-center justify-between">
                  <a href={`/address/${validator.address}`} className="text-[#ffa729] hover:underline font-mono">
                    {`${validator.address.slice(0, 8)}...${validator.address.slice(-6)}`}
                  </a>
                  <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${
                    validator.isActive ? 'bg-green-900/20 text-green-400' : 'bg-red-900/20 text-red-400'
                  }`}>
                    {validator.isActive ? 'Active' : 'Inactive'}
                  </span>
                </div>
              </summary>
              <div className="px-4 py-2 space-y-2 bg-[#2d2d2d]/30">
                <div className="flex justify-between">
                  <span className="text-gray-400">Age:</span>
                  <span className="text-gray-300">{epochsToDays(validator.age).toFixed(1)} days</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Uptime:</span>
                  <span className="text-gray-300">{validator.uptime.toFixed(2)}%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Staked:</span>
                  <span className="text-gray-300 font-mono">
                    {formatAmount(validator.stakedAmount)[0]} {formatAmount(validator.stakedAmount)[1]}
                  </span>
                </div>
              </div>
            </details>
          ))}
        </div>
      </div>

      {/* Pagination Controls */}
      <div className="mt-4 flex flex-wrap justify-center items-center gap-2">
        <button
          onClick={() => goToPage(currentPage - 1)}
          disabled={currentPage === 1}
          className={`px-3 py-1 rounded-lg text-sm ${
            currentPage === 1
              ? 'bg-[#2d2d2d]/30 text-gray-500 cursor-not-allowed'
              : 'bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d]'
          }`}
        >
          Previous
        </button>
        
        {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
          let pageNum;
          if (totalPages <= 5) {
            pageNum = i + 1;
          } else if (currentPage <= 3) {
            pageNum = i + 1;
          } else if (currentPage >= totalPages - 2) {
            pageNum = totalPages - 4 + i;
          } else {
            pageNum = currentPage - 2 + i;
          }
          
          return (
            <button
              key={i}
              onClick={() => goToPage(pageNum)}
              className={`w-8 h-8 rounded-lg text-sm ${
                currentPage === pageNum
                  ? 'bg-[#ffa729] text-black'
                  : 'bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d]'
              }`}
            >
              {pageNum}
            </button>
          );
        })}
        
        <button
          onClick={() => goToPage(currentPage + 1)}
          disabled={currentPage === totalPages}
          className={`px-3 py-1 rounded-lg text-sm ${
            currentPage === totalPages
              ? 'bg-[#2d2d2d]/30 text-gray-500 cursor-not-allowed'
              : 'bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d]'
          }`}
        >
          Next
        </button>
      </div>
    </div>
  );
}
