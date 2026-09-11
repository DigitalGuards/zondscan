import { createServer } from 'node:http';

// Synthetic data for local interface tests. No node, wallet, or network connection.
const hex = (value) => `0x${BigInt(value).toString(16)}`;
const hash = (value) => `0x${value.toString(16).padStart(64, '0')}`;
const sender = `Q${'a'.repeat(40)}`;
const recipient = `Q${'b'.repeat(40)}`;
const timestamp = 1789043400;
const amounts = [
  '0.01',
  '0.000000000000000016',
  '0.05',
  '0',
  '0.000001',
  '1.999999999999999999',
  '1000.123456789123456789',
  '0.25',
  '3.3',
  '200',
];
const txs = amounts.map((amount, index) => ({
  TxHash: hash(index + 1),
  From: sender,
  To: recipient,
  Amount: amount,
  TimeStamp: timestamp - index * 3600,
  BlockNumber: String(233635 - index),
  InOut: 0,
  TxType: 2,
  PaidFees: 0.000021,
}));
const blocks = txs.map((tx, index) => ({
  number: hex(233635 - index),
  timestamp: hex(tx.TimeStamp),
  hash: hash(100 + index),
  parentHash: hash(101 + index),
  miner: sender,
  gasUsed: '0x5208',
  gasLimit: '0x1312d00',
  baseFeePerGas: '0x1',
  size: '0x400',
  transactions: [{ hash: tx.TxHash, from: sender, to: recipient, value: '0x0' }],
  prevRandao: hash(200),
  stateRoot: hash(201),
  receiptsRoot: hash(202),
  transactionsRoot: hash(203),
  extraData: '0x74657374',
  withdrawals: [],
}));
const transfers = [
  { amount: '0', tokenStandard: 'ERC-20', tokenSymbol: 'ZERO', tokenName: 'Zero demo' },
  {
    amount: '1250000000000000000',
    tokenStandard: 'ERC-20',
    tokenSymbol: 'DEMO',
    tokenName: 'Demo token',
  },
  {
    amount: '0',
    tokenStandard: 'ERC-721',
    tokenSymbol: 'NFT',
    tokenName: 'Demo NFT',
    tokenID: '0',
  },
].map((transfer, index) => ({
  contractAddress: `Q${String(index + 1).repeat(40)}`,
  from: sender,
  to: recipient,
  tokenDecimals: 18,
  timestamp: hex(timestamp),
  blockNumber: hex(233635),
  txHash: hash(1),
  logIndex: hex(index),
  transferType: 'transfer',
  ...transfer,
}));
const overview = {
  currentPrice: 0.712583,
  priceChange24h: 4.43975,
  circulating: '80388299',
  marketcap: 55862008,
  validatorCount: 512,
  countwallets: 459,
  contractCount: 161,
  status: { dataInitialized: true, syncing: false },
  tradingVolume: 37664,
};
const gas = {
  avgGasPriceHex: '0x430e234e',
  avgGasUsedHex: '0x5208',
  avgGasLimitHex: '0x1312d00',
  avgBlockTimeSec: 60,
  pendingCount: 0,
  lastBlockNumberHex: hex(233635),
  lastGasUsedHex: '0x5208',
  lastGasLimitHex: '0x1312d00',
  gasPriceHistogram: [],
  qrlUsdPrice: overview.currentPrice,
};

createServer((request, response) => {
  const path = new URL(request.url, 'http://127.0.0.1').pathname;
  let data = {
    '/health': { ok: true },
    '/overview': overview,
    '/gas/summary': gas,
    '/txs': { txs, total: txs.length },
    '/transactions': { response: txs },
    '/latestblock': { blockNumber: 233700, qrlUsdPrice: overview.currentPrice },
    '/blocks': { blocks, total: 233700 },
    '/epoch': {
      headEpoch: '1825',
      headSlot: '233635',
      finalizedEpoch: '1823',
      justifiedEpoch: '1824',
      slotsPerEpoch: 128,
      secondsPerSlot: 60,
      slotInEpoch: 35,
      timeToNextEpoch: 5580,
      updatedAt: timestamp,
    },
    '/epochs': {
      epochs: [
        {
          epoch: 1825,
          timestamp,
          totalStaked: '8000000000000000000000000',
          proposedBlocks: 120,
          missedBlocks: 8,
          participationRate: 96.7,
        },
      ],
      total: 1826,
    },
    '/gas/history': {
      range: '24h',
      data: blocks.map((block) => ({
        blockNumber: block.number,
        timestamp: Number(block.timestamp),
        gasUsed: block.gasUsed,
        gasLimit: block.gasLimit,
        baseFeePerGas: '0x1',
        txCount: 1,
      })),
    },
  }[path];
  if (path.startsWith('/tx/'))
    data = {
      response: {
        TxHash: hash(1),
        From: sender,
        To: recipient,
        Value: '0x0',
        BlockNumber: hex(233635),
        BlockTimestamp: hex(timestamp),
        GasUsed: '0x5208',
        GasPrice: '0x3b9aca00',
        Nonce: '0x1',
      },
      latestBlock: 233640,
      tokenTransfers: transfers,
      input: '0x12345678',
      logs: [],
      internalTransactions: [],
      receiptStatus: '0x1',
    };
  if (path.startsWith('/block/')) data = { block: { result: blocks[0] } };
  if (path.startsWith('/address/aggregate/'))
    data = {
      address: { balance: 123.45 },
      rank: 7,
      first_seen: timestamp - 86400,
      last_seen: timestamp,
      transactions_by_address: txs,
      transactions_count: txs.length,
      internal_transactions_by_address: [],
      contract_code: null,
    };
  if (/^\/address\/[^/]+\/token-transfers$/.test(path))
    data = { transfers, total: transfers.length };
  if (/^\/address\/[^/]+\/transactions$/.test(path))
    data = { transactions: txs, total: txs.length };
  if (/^\/address\/[^/]+\/internal-transactions$/.test(path)) data = { transactions: [], total: 0 };
  response.writeHead(data ? 200 : 404, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': 'http://127.0.0.1:18090',
  });
  response.end(JSON.stringify(data ?? { error: 'No local fixture for this endpoint' }));
}).listen(18091, '127.0.0.1', () => console.log('Local fixture API: http://127.0.0.1:18091'));
