import { notFound } from 'next/navigation';
import TransactionView from './transaction-view';
import type { TransactionDetails } from './types';
import { decodeBase64ToHexadecimal } from '../../lib/server-helpers';

interface PageProps {
  params: Promise<{ query: string }>;
}

async function getTransaction(txHash: string): Promise<TransactionDetails> {
  // Validate transaction hash format
  const hashRegex = /^0x[0-9a-fA-F]{64}$/;
  if (!hashRegex.test(txHash)) {
    throw new Error('Invalid transaction hash format');
  }

  console.log('Fetching transaction:', txHash);
  const response = await fetch(`${process.env.NEXT_PUBLIC_HANDLER_URL}/tx/${txHash}`, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    next: { revalidate: 60 }, // Cache for 60 seconds
  });

  if (!response.ok) {
    if (response.status === 404) {
      notFound();
    }
    throw new Error('Failed to fetch transaction details');
  }

  const data = await response.json();
  console.log('Raw API response:', JSON.stringify(data, null, 2));

  if (!data.response) {
    notFound();
  }

  const txData = data.response;
  console.log('Transaction data:', JSON.stringify(txData, null, 2));

  // Helper function to handle address decoding
  const decodeAddress = (address: string | null | undefined): string => {
    console.log('Decoding address:', address);
    if (!address) return '';
    // Check if it's already a hex address (starts with 0x)
    if (address.startsWith('0x')) {
      console.log('Address is already hex:', address);
      return address;
    }
    // Otherwise decode from base64
    try {
      const decoded = decodeBase64ToHexadecimal(address);
      console.log('Decoded address:', decoded);
      return decoded; // decodeBase64ToHexadecimal already adds '0x' prefix
    } catch (error) {
      console.error('Error decoding address:', error);
      return address;
    }
  };

  // Helper function to handle value conversion
  const formatValue = (value: string | number | null | undefined, valueStr?: string): string => {
    console.log('Formatting value:', value, 'valueStr:', valueStr);
    
    // If we have a hex string value from backend, use it directly
    if (valueStr && valueStr.startsWith('0x')) {
      console.log('Using valueStr:', valueStr);
      return valueStr;
    }
    
    if (value === null || value === undefined) return '0x0';
    
    // If it's a number (uint64 from backend), convert to hex string
    if (typeof value === 'number') {
      // Convert to hex string with proper scaling (multiply by 10^18)
      const valueStr = value.toString();
      const scaledValue = BigInt(valueStr) * BigInt('1000000000000000000');
      const hexValue = '0x' + scaledValue.toString(16);
      console.log('Converted number to hex:', hexValue);
      return hexValue;
    }
    
    // If it's already a hex string, keep it as is
    if (typeof value === 'string' && value.startsWith('0x')) {
      console.log('Value is already hex:', value);
      return value;
    }
    
    // If it's a string containing a number
    if (typeof value === 'string' && /^\d+$/.test(value)) {
      // Convert to hex string with proper scaling (multiply by 10^18)
      const scaledValue = BigInt(value) * BigInt('1000000000000000000');
      const hexValue = '0x' + scaledValue.toString(16);
      console.log('Converted string number to hex:', hexValue);
      return hexValue;
    }
    
    // If it's base64, decode it
    if (typeof value === 'string') {
      try {
        const decoded = decodeBase64ToHexadecimal(value);
        console.log('Decoded value:', decoded);
        return decoded; // decodeBase64ToHexadecimal already adds '0x' prefix
      } catch (error) {
        console.error('Error decoding value:', error);
        return value;
      }
    }
    
    return '0x0';
  };

  // Helper function to ensure integer values
  const ensureInteger = (value: any): number => {
    if (typeof value === 'number') {
      return Math.floor(value);
    }
    if (typeof value === 'string') {
      return Math.floor(parseFloat(value));
    }
    return 0;
  };

  const transaction = {
    hash: txData.hash || txHash,
    blockNumber: ensureInteger(txData.blockNumber) || '',
    from: decodeAddress(txData.from),
    to: decodeAddress(txData.to),
    value: formatValue(txData.value, txData.valueStr),
    timestamp: ensureInteger(txData.blockTimestamp || txData.timestamp) || 0,
    gasUsed: txData.gasUsedStr || '0x0',
    gasPrice: txData.gasPriceStr || '0x0',
    nonce: ensureInteger(txData.nonce),
    latestBlock: ensureInteger(data.latestBlock)
  };

  console.log('Processed transaction:', JSON.stringify(transaction, null, 2));

  return transaction;
}

export default async function TransactionPage({ params }: PageProps): Promise<JSX.Element> {
  try {
    const resolvedParams = await params;
    console.log('Transaction hash from params:', resolvedParams.query);
    const transaction = await getTransaction(resolvedParams.query);
    return <TransactionView transaction={transaction} />;
  } catch (error) {
    console.error('Error in TransactionPage:', error);
    return (
      <div className="py-8">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl" role="alert">
          <strong className="font-bold">Error: </strong>
          <span className="block sm:inline">
            {error instanceof Error ? error.message : 'Failed to load transaction details'}
          </span>
        </div>
      </div>
    );
  }
}
