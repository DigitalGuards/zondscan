'use client'

import React from 'react'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon } from '@heroicons/react/20/solid'

interface Endpoint {
  method: string
  path: string
  description: string
  params?: string
  example?: string
}

interface EndpointGroup {
  category: string
  endpoints: Endpoint[]
}

const endpointGroups: EndpointGroup[] = [
  {
    category: 'Overview & Market Data',
    endpoints: [
      {
        method: 'GET',
        path: '/overview',
        description: 'Returns network overview including market cap, current price, wallet count, circulating supply, daily transaction volume, validator count, contract count, and 24h trading volume.',
        example: '/overview',
      },
      {
        method: 'GET',
        path: '/price-history',
        description: 'Returns historical price data for charts. Supports intervals: 4h, 12h, 24h, 7d, 30d, all.',
        params: 'interval (query) - Time interval (default: 24h)',
        example: '/price-history?interval=7d',
      },
    ],
  },
  {
    category: 'Blocks',
    endpoints: [
      {
        method: 'GET',
        path: '/blocks',
        description: 'Returns a paginated list of the latest blocks.',
        params: 'page (query, default: 1), limit (query, default: 5, max: 100)',
        example: '/blocks?page=1&limit=10',
      },
      {
        method: 'GET',
        path: '/block/:number',
        description: 'Returns details for a specific block by its number. Accepts decimal or 0x-prefixed hex.',
        example: '/block/12345',
      },
      {
        method: 'GET',
        path: '/latestblock',
        description: 'Returns the latest synced block number.',
        example: '/latestblock',
      },
      {
        method: 'GET',
        path: '/blocksizes',
        description: 'Returns block size history for chart visualizations.',
        example: '/blocksizes',
      },
    ],
  },
  {
    category: 'Transactions',
    endpoints: [
      {
        method: 'GET',
        path: '/transactions',
        description: 'Returns the latest transactions across the network.',
        example: '/transactions',
      },
      {
        method: 'GET',
        path: '/txs',
        description: 'Returns paginated transactions across the network, including total count and latest block number.',
        params: 'page (query) - Page number (required)',
        example: '/txs?page=1',
      },
      {
        method: 'GET',
        path: '/tx/:hash',
        description: 'Returns detailed transaction data by hash, including contract creation info and token transfer details if applicable.',
        example: '/tx/0xabc123...',
      },
      {
        method: 'GET',
        path: '/pending-transactions',
        description: 'Returns paginated pending (mempool) transactions.',
        params: 'page (query, default: 1), limit (query, default: 10, max: 100)',
        example: '/pending-transactions?page=1&limit=10',
      },
      {
        method: 'GET',
        path: '/pending-transaction/:hash',
        description: 'Returns a specific pending transaction by hash. Returns mined status if the transaction has been confirmed.',
        example: '/pending-transaction/0xabc123...',
      },
    ],
  },
  {
    category: 'Addresses',
    endpoints: [
      {
        method: 'GET',
        path: '/address/aggregate/:address',
        description: 'Returns comprehensive data for an address: balance, transaction count, rank, all transactions, internal transactions, contract code, and latest block number.',
        example: '/address/aggregate/0xabc123...',
      },
      {
        method: 'GET',
        path: '/address/:address/transactions',
        description: 'Returns paginated non-zero transactions for an address.',
        params: 'page (query, default: 1), limit (query, default: 5, max: 100)',
        example: '/address/0xabc123.../transactions?page=1&limit=10',
      },
      {
        method: 'GET',
        path: '/address/:address/tokens',
        description: 'Returns all token balances held by an address. Useful for wallet integration and token auto-discovery.',
        example: '/address/0xabc123.../tokens',
      },
      {
        method: 'POST',
        path: '/getBalance',
        description: 'Returns the balance for an address.',
        params: 'address (form data) - The address to query',
      },
      {
        method: 'GET',
        path: '/richlist',
        description: 'Returns the richlist - top addresses by balance.',
        example: '/richlist',
      },
      {
        method: 'GET',
        path: '/walletdistribution/:count',
        description: 'Returns wallet distribution data for analytics.',
        example: '/walletdistribution/100',
      },
    ],
  },
  {
    category: 'Validators & Epochs',
    endpoints: [
      {
        method: 'GET',
        path: '/validators',
        description: 'Returns the list of validators with pagination support via page tokens.',
        params: 'page_token (query) - Token for next page of results',
        example: '/validators',
      },
      {
        method: 'GET',
        path: '/validator/:id',
        description: 'Returns detailed information for a specific validator by ID.',
        example: '/validator/1',
      },
      {
        method: 'GET',
        path: '/validators/stats',
        description: 'Returns aggregate validator statistics.',
        example: '/validators/stats',
      },
      {
        method: 'GET',
        path: '/validators/history',
        description: 'Returns historical validator count data for chart visualizations.',
        params: 'limit (query, default: 100, max: 100)',
        example: '/validators/history?limit=50',
      },
      {
        method: 'GET',
        path: '/epochs',
        description: 'Returns a paginated list of epochs.',
        params: 'page (query, default: 1), limit (query, default: 15, max: 100)',
        example: '/epochs?page=1&limit=15',
      },
      {
        method: 'GET',
        path: '/epoch/:id',
        description: 'Returns detailed information for a specific epoch.',
        example: '/epoch/42',
      },
      {
        method: 'GET',
        path: '/epoch',
        description: 'Returns information about the current epoch.',
        example: '/epoch',
      },
    ],
  },
  {
    category: 'Smart Contracts & Tokens',
    endpoints: [
      {
        method: 'GET',
        path: '/contracts',
        description: 'Returns a paginated list of smart contracts. Supports search and token filtering.',
        params: 'page (query, default: 0), limit (query, default: 10, max: 100), search (query), isToken (query, true/false)',
        example: '/contracts?page=0&limit=10&isToken=true',
      },
      {
        method: 'GET',
        path: '/token/:address/info',
        description: 'Returns summary information for a token contract (name, symbol, decimals, supply).',
        example: '/token/0xabc123.../info',
      },
      {
        method: 'GET',
        path: '/token/:address/holders',
        description: 'Returns a paginated list of token holders for a given contract.',
        params: 'page (query, default: 0), limit (query, default: 25, max: 100)',
        example: '/token/0xabc123.../holders?page=0&limit=25',
      },
      {
        method: 'GET',
        path: '/token/:address/transfers',
        description: 'Returns a paginated list of token transfers for a given contract.',
        params: 'page (query, default: 0), limit (query, default: 25, max: 100)',
        example: '/token/0xabc123.../transfers?page=0&limit=25',
      },
    ],
  },
]

const BASE_URL = 'https://zondscan.com/api'

function classNames(...classes: string[]): string {
  return classes.filter(Boolean).join(' ')
}

function MethodBadge({ method }: { method: string }) {
  const colors: Record<string, string> = {
    GET: 'bg-green-900/50 text-green-400 border-green-700',
    POST: 'bg-blue-900/50 text-blue-400 border-blue-700',
  }
  return (
    <span className={`inline-block px-2 py-0.5 text-xs font-mono font-bold rounded border ${colors[method] || 'bg-gray-700 text-gray-300 border-gray-600'}`}>
      {method}
    </span>
  )
}

export default function ApiExplorerClient(): JSX.Element {
  return (
    <div className="min-h-screen bg-[#1a1a1a] text-gray-300 p-4 md:p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl md:text-3xl font-bold mb-2 text-[#ffa729]">
          API Explorer
        </h1>
        <p className="text-gray-400 mb-2">
          The Zondscan API provides free, open access to QRL Zond blockchain data. No API key required.
        </p>
        <p className="text-gray-500 text-sm mb-8">
          As the world transitions to post-quantum cryptography, we believe blockchain data should remain freely accessible to everyone. This API will always be free to use.
        </p>

        <div className="bg-[#2d2d2d] rounded-lg p-4 mb-8">
          <h2 className="text-sm font-semibold text-gray-400 mb-2">Base URL</h2>
          <code className="text-[#ffa729] text-sm font-mono break-all">{BASE_URL}</code>
        </div>

        <div className="space-y-4">
          {endpointGroups.map((group, groupIndex) => (
            <Disclosure as="div" key={groupIndex} defaultOpen={groupIndex === 0}>
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between px-4 py-3 bg-[#2d2d2d] rounded-lg text-left hover:bg-[#333] transition-colors">
                    <span className="text-sm md:text-base font-semibold text-[#ffa729]">
                      {group.category}
                      <span className="ml-2 text-xs text-gray-500 font-normal">
                        ({group.endpoints.length} endpoint{group.endpoints.length !== 1 ? 's' : ''})
                      </span>
                    </span>
                    <ChevronDownIcon
                      className={classNames(
                        open ? 'rotate-180' : '',
                        'h-5 w-5 text-[#ffa729] transition-transform duration-200'
                      )}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="mt-1 space-y-2">
                    {group.endpoints.map((endpoint, endpointIndex) => (
                      <div
                        key={endpointIndex}
                        className="bg-[#262626] rounded-lg p-4 border border-[#3d3d3d]"
                      >
                        <div className="flex items-center gap-3 mb-2">
                          <MethodBadge method={endpoint.method} />
                          <code className="text-sm font-mono text-gray-200 break-all">
                            {endpoint.path}
                          </code>
                        </div>
                        <p className="text-sm text-gray-400 mb-2">{endpoint.description}</p>
                        {endpoint.params && (
                          <div className="mb-2">
                            <span className="text-xs font-semibold text-gray-500 uppercase">Parameters: </span>
                            <span className="text-xs text-gray-400">{endpoint.params}</span>
                          </div>
                        )}
                        {endpoint.example && (
                          <div className="mt-2 bg-[#1a1a1a] rounded px-3 py-2 border border-[#3d3d3d]">
                            <span className="text-xs font-semibold text-gray-500 uppercase block mb-1">Example</span>
                            <code className="text-xs font-mono text-green-400 break-all">
                              {BASE_URL}{endpoint.example}
                            </code>
                          </div>
                        )}
                      </div>
                    ))}
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>
          ))}
        </div>

        <div className="mt-8 bg-[#2d2d2d] rounded-lg p-4 border border-[#3d3d3d]">
          <h2 className="text-sm font-semibold text-[#ffa729] mb-2">Notes</h2>
          <ul className="text-sm text-gray-400 space-y-1 list-disc list-inside">
            <li>All responses are returned in JSON format</li>
            <li>Numeric blockchain values (balances, block numbers) are typically hex-encoded with a 0x prefix</li>
            <li>Pagination uses <code className="text-gray-300">page</code> and <code className="text-gray-300">limit</code> query parameters (max limit: 100)</li>
            <li>Addresses accept both 0x and Z-prefixed formats</li>
            <li>No authentication or API key is required</li>
            <li>Rate limiting may apply to prevent abuse</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
