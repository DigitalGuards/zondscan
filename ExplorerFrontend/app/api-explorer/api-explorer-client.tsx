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
        example: '/address/aggregate/Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1',
      },
      {
        method: 'GET',
        path: '/address/:address/transactions',
        description: 'Returns paginated non-zero transactions for an address.',
        params: 'page (query, default: 1), limit (query, default: 5, max: 100)',
        example: '/address/Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1/transactions?page=1&limit=10',
      },
      {
        method: 'GET',
        path: '/address/:address/tokens',
        description: 'Returns all token balances held by an address. Useful for wallet integration and token auto-discovery. Phase 2: NFT holdings expand to one row per (collection, tokenID), each row carries `tokenStandard` and (for NFTs) `tokenID`. ERC-20 rows continue to return one row per collection with the existing balance string. Phase 3b adds an optional `?standard=` filter: pass `ERC-20` to scope to fungibles (wallets should do this when populating an ERC-20-aware Tokens card so NFT rows do not get treated as zero-balance fungibles).',
        params: 'standard (query, optional, ERC-20|ERC-721|ERC-1155, invalid value → 400)',
        example: '/address/Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1/tokens?standard=ERC-20',
      },
      {
        method: 'GET',
        path: '/address/:address/nfts',
        description: 'Phase 3b: per-(contract, tokenID) NFT holdings for an address, joined with both the collection-level contractCode row (collectionName, collectionSymbol) and the per-tokenID tokenMetadata row (name, image, description, attributes). Designed for the wallet\'s "Add NFT" picker so a single response is enough to render thumbnails + names without follow-up roundtrips. Optional `?standard=ERC-721|ERC-1155` to scope further; default returns both NFT standards. ERC-20 is rejected with 400, use /address/:addr/tokens?standard=ERC-20 for that.',
        params: 'standard (query, optional, ERC-721|ERC-1155, invalid value → 400)',
        example: '/address/Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1/nfts',
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
        description: 'Returns a paginated list of smart contracts. Supports search, token/standard filtering, and substring match on name, symbol, or off-chain metadataName. The `standard` filter (Phase 1 NFT support) returns only contracts classified as the given EIP standard; `isToken=true` is the looser superset (matches all three token standards). Phase 3a: each row includes optional `metadataName` / `metadataImage` / `metadataDescription` / `metadataExternalURL` populated from the contract\'s `contractURI()` getter via the IPFS gateway.',
        params: 'page (query, default: 0), limit (query, default: 10, max: 100), search (query, matches address, creator address, name, symbol, or metadataName, case-insensitive), isToken (query, true/false), standard (query, ERC-20|ERC-721|ERC-1155, invalid value → 400)',
        example: '/contracts?page=0&limit=10&standard=ERC-721',
      },
      {
        method: 'GET',
        path: '/token/:address/info',
        description: 'Returns summary information for a token contract (name, symbol, decimals, supply). Phase 3a: the underlying contract document also carries `metadataName` / `metadataImage` / `metadataDescription` / `metadataExternalURL` resolved from `contractURI()`, available on the /contracts list rows and on the address-page contract document.',
        example: '/token/Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147/info',
      },
      {
        method: 'GET',
        path: '/token/:address/holders',
        description: 'Returns a paginated list of token holders for a given contract. ERC-20 returns one row per (contract, holder). For ERC-721/1155, holders are aggregated across all ids by default (one row per distinct holder, balance is the cross-id total). Pass `tokenID=<decimal>` to filter to a single id (one row per holder of that id, balance is per-id quantity).',
        params: 'page (query, default: 0), limit (query, default: 25, max: 100), tokenID (query, optional, decimal uint256 NFT id filter)',
        example: '/token/Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147/holders?page=0&limit=25&tokenID=1',
      },
      {
        method: 'GET',
        path: '/token/:address/tokens',
        description: 'Phase 2 + 3b: returns the distinct tokenID list minted on an NFT contract with holder count per id. Each row also carries `name` + `image` + `description` from the per-token off-chain metadata when the metadata fetcher has resolved it (otherwise those fields are absent and the UI falls back to a "#<id>" label). Empty list for ERC-20 contracts.',
        params: 'page (query, default: 0), limit (query, default: 25, max: 100)',
        example: '/token/Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147/tokens?page=0&limit=25',
      },
      {
        method: 'GET',
        path: '/token/:address/:id',
        description: 'Phase 3b: per-token metadata document for one (contract, tokenID) tuple. Returns the resolved name / description / image / external_url plus the OpenSea-style attribute list. 400 if id is not a decimal integer, 404 if the token has no stub yet (contract isn\'t an NFT, or that id has never been transferred).',
        example: '/token/Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147/1',
      },
      {
        method: 'GET',
        path: '/token/:address/transfers',
        description: 'Returns a paginated list of token transfers for a given contract. Each row carries `tokenStandard` (ERC-20|ERC-721|ERC-1155) and `tokenID` (decimal uint256, empty for ERC-20). ERC-1155 TransferBatch logs are fanned out to one row per (id, value) tuple, so a batch of N items produces N rows that share txHash + logIndex but differ on tokenID.',
        params: 'page (query, default: 0), limit (query, default: 25, max: 100)',
        example: '/token/Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147/transfers?page=0&limit=25',
      },
    ],
  },
  {
    category: 'Contract Verification',
    endpoints: [
      {
        method: 'GET',
        path: '/contract/compiler-info',
        description: 'Returns the language and pinned Hyperion build id the verifier is willing to accept. Use this to confirm the explorer can verify against your hypc version before you POST a job. Returns 503 when the verifier is not configured on this deployment.',
        example: '/contract/compiler-info',
      },
      {
        method: 'POST',
        path: '/contract/verify',
        description: 'Enqueues a verification job for a deployed contract. The endpoint compiles the supplied source via the pinned hypc runner, compares the deployed-bytecode (Solidity-style CBOR metadata trailer stripped on both sides) and on success writes the verified source + ABI back onto the contract document. Rate-limited per IP. Max body 1 MiB. Returns { jobId, status, address } synchronously; poll /contract/verify/:jobId for the terminal state.',
        params: 'JSON body, required: address, sourceCode, contractName. Optional: compilerVersion, optimizerEnabled, optimizerRuns, evmVersion, constructorArguments, libraries, imports, license. Example body: {"address":"Q…","sourceCode":"...","contractName":"Foo"}',
        example: '/contract/verify',
      },
      {
        method: 'GET',
        path: '/contract/verify/:jobId',
        description: 'Returns the current state of a verification job. Status values: pending, compiling, success, failed. The body echoes the original submission payload (sans the standard-JSON wrapper) and, on success, includes a result reference with the ABI and verifiedAt stamp.',
        example: '/contract/verify/27fcbb55ffb611ec',
      },
    ],
  },
  {
    category: 'Contract Interaction',
    endpoints: [
      {
        method: 'POST',
        path: '/contract/call',
        description: 'Open-but-bounded eth_call proxy for read-only contract reads. The body is forwarded as a qrl_call to the node with a hard gas cap, data-size cap and per-call timeout. The `to` address must be a known contract in the indexer (404 otherwise) so non-contract reads cannot be proxied. Returns { result } on success or { error, code, reverted: true } on RPC errors (HTTP 200 in both branches, distinguish via fields). Per-IP rate-limited (60/min).',
        params: 'JSON body, to (Q-address, required), data (0x-prefixed even-length hex, required, max 8 KiB). Example body: {"to":"Qed2af...","data":"0xef690cc0"}',
        example: '/contract/call',
      },
      {
        method: 'POST',
        path: '/contract/explain/:address',
        description: 'On-demand AI summary of a VERIFIED contract’s source code (Claude Haiku). The summary is cached per-contract in the contractCode collection (fields aiExplanation / aiExplanationAt / aiExplanationModel) so subsequent reads are free until ?regenerate=1 busts the cache. Returns 403 if the address is not a verified contract, only verified contracts can be analysed. Regenerations are capped at 5 per contract per rolling 7-day window (429 when exceeded).',
        params: 'regenerate (query, optional), "1" or "true" to force a fresh LLM call (counts against the 5/week cap)',
        example: '/contract/explain/Qed2af55af7a492a6e504b364dd882159f9374f46',
      },
    ],
  },
]

const BASE_URL = 'https://zondscan.com/api'

import MethodBadge from '../components/MethodBadge'

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
            <Disclosure as="div" key={groupIndex}>
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
                      className={(open ? 'rotate-180' : '') + ' h-5 w-5 text-[#ffa729] transition-transform duration-200'}
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
            <li>Addresses are Q-prefixed (canonical QRL 2.0 form), e.g. <code className="text-gray-300">Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1</code></li>
            <li>No authentication or API key is required</li>
            <li>Rate limiting may apply to prevent abuse</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
