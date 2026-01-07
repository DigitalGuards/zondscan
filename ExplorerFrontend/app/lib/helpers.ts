export function decodeToHex(input: string, format?: string): string {
  const decoded = atob(input);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export function toFixed(x: number | string | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }

  // Convert to number if it's a string
  const num = typeof x === 'string' ? parseFloat(x) : x;

  // Check if it's a valid number
  if (isNaN(num)) {
    return "0";
  }

  if (Math.abs(num) < 1.0) {
    const e = parseInt(num.toString().split('e-')[1]);
    if (e) {
      const val = num * Math.pow(10, e - 1);
      return '0.' + (new Array(e)).join('0') + val.toString().substring(2);
    }
  } else {
    const e = parseInt(num.toString().split('+')[1]);
    if (e > 20) {
      const adjustedE = e - 20;
      const val = num / Math.pow(10, adjustedE);
      return val + (new Array(adjustedE + 1)).join('0');
    }
  }
  return num.toString();
}

export function formatGas(amount: number | string | undefined | null): [string, string] {
  // Handle undefined or null
  if (amount === undefined || amount === null) {
    return ['0', 'Shor'];
  }

  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0', 'Shor'];
  }

  try {
    let value: number | bigint;
    // Handle hex strings (e.g., "0x123")
    if (typeof amount === 'string' && amount.startsWith('0x')) {
      value = parseInt(amount, 16);
    }
    // Handle number values
    else if (typeof amount === 'number') {
      value = amount;
    }
    // Handle other formats
    else {
      value = BigInt(amount);
    }

    // Return the numeric value as a string with 'Shor' unit
    return [value.toString(), 'Shor'];
  } catch (error) {
    console.error('Error converting gas amount:', error, amount);
    return ['0', 'Shor'];
  }
}

export function formatAmount(amount: number | string | undefined | null): [string, string] {
  // Handle undefined or null
  if (amount === undefined || amount === null) {
    return ['0.00', 'QRL'];
  }

  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0.00', 'QRL'];
  }

  let totalNum: number;
  try {
    // Handle hex strings (e.g., "0x123") from node
    if (typeof amount === 'string' && amount.startsWith('0x')) {
      const value = BigInt(amount);
      const divisor = BigInt('1000000000000000000'); // 10^18
      const wholePart = value / divisor;
      const fractionalPart = value % divisor;
      totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
    }
    // Handle decimal numbers (convert to wei/shor format first)
    else if (typeof amount === 'number' || (typeof amount === 'string' && !isNaN(Number(amount)))) {
      const floatValue = parseFloat(String(amount));
      if (floatValue < 1000000000000000000) { // If number is already in QRL format
        totalNum = floatValue;
      } else {
        const value = BigInt(Math.floor(floatValue));
        const divisor = BigInt('1000000000000000000'); // 10^18
        const wholePart = value / divisor;
        const fractionalPart = value % divisor;
        totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
      }
    }
    // Handle other formats (assuming they're in wei/shor)
    else {
      throw new Error('Invalid amount format');
    }
  } catch (error) {
    console.error('Error converting amount:', error, amount);
    return ['0.00', 'QRL'];
  }

  // Format with appropriate decimal places, avoiding scientific notation
  if (totalNum === 0) {
    return ['0.00', 'QRL'];
  } else if (totalNum < 0.000001) {
    // For very small numbers, show all significant digits without trailing zeros
    return [totalNum.toFixed(18).replace(/\.?0+$/, ''), 'QRL'];
  } else if (totalNum < 1) {
    // For numbers less than 1, show up to 6 decimal places
    return [totalNum.toFixed(6).replace(/\.?0+$/, ''), 'QRL'];
  } else if (totalNum < 1000) {
    // For numbers between 1 and 999, show up to 4 decimal places
    return [totalNum.toFixed(4).replace(/\.?0+$/, ''), 'QRL'];
  } else {
    // For large numbers, show 2 decimal places
    return [totalNum.toFixed(2).replace(/\.?0+$/, ''), 'QRL'];
  }
}

export function normalizeHexString(hexData: string | undefined | null): string {
  if (!hexData) return '';

  // If it starts with 0x, remove the prefix
  if (typeof hexData === 'string' && hexData.startsWith('0x')) {
    return hexData.slice(2);
  }

  // If it starts with Z, remove the prefix
  if (typeof hexData === 'string' && hexData.startsWith('Z')) {
    return hexData.slice(1);
  }

  // If it's a valid hex string without prefix, return as is
  if (typeof hexData === 'string' && /^[0-9a-fA-F]+$/.test(hexData)) {
    return hexData;
  }

  console.error('Invalid hex string:', hexData);
  return '';
}

export function epochToISO(timestamp: number | undefined | null): string {
  if (!timestamp) return '1970-01-01';
  const date = new Date(timestamp * 1000);
  const datePart = date.toISOString().split('T')[0];
  return datePart;
}

export function formatTimestamp(timestamp: number | undefined | null): string {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  // Use UTC to avoid hydration mismatch between server and client
  const day = date.getUTCDate();
  const month = date.getUTCMonth() + 1;
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours();
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  const ampm = hours >= 12 ? 'PM' : 'AM';
  const hour12 = hours % 12 || 12;
  return `${month}/${day}/${year}, ${hour12}:${minutes}:${seconds} ${ampm} UTC`;
}

export function formatNumber(value: number): string {
  if (typeof value !== "number" || isNaN(value)) {
    return "Error";
  }
  let formatted: string;
  if (value >= 1e12) {
    formatted = (value / 1e12).toFixed(2) + 'T';
  } else if (value >= 1e9) {
    formatted = (value / 1e9).toFixed(2) + 'B';
  } else if (value >= 1e6) {
    formatted = (value / 1e6).toFixed(2) + 'M';
  } else if (value >= 1e3) {
    formatted = (value / 1e3).toFixed(2) + 'K';
  } else {
    formatted = value.toFixed(2);
  }
  return '$' + formatted;
}

export function formatNumberWithCommas(x: number | string | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

export function epochsToDays(epochs: number): number {
  // Each epoch is 128 slots
  // Each slot takes 60 seconds
  // So each epoch is 128 * 60 seconds
  // Convert to days
  return (epochs * 128 * 60) / (24 * 60 * 60);
}

export function truncateHash(hash: string | undefined | null, startLength = 6, endLength = 4): string {
  if (!hash || hash.length < startLength + endLength) return hash || '';
  return `${hash.slice(0, startLength)}...${hash.slice(-endLength)}`;
}

/**
 * Formats an address to ensure it has the correct prefix (Z for QRL addresses, 0x for contract addresses)
 */
export function formatAddress(address: string | undefined | null): string {
  if (!address) return '';

  // If already has Z prefix, return as is
  if (address.startsWith('Z')) {
    return address;
  }

  // If has 0x prefix
  if (address.startsWith('0x')) {
    // For contract addresses (starting with 0x7), keep the 0x prefix
    if (address.startsWith('0x7')) {
      return address;
    }
    // For regular addresses, convert to Z prefix
    return 'Z' + address.slice(2);
  }

  // If no prefix but is a valid hex string, add Z prefix
  if (/^[0-9a-fA-F]+$/.test(address)) {
    return 'Z' + address;
  }

  // If invalid format, return as is
  return address;
}

/**
 * Decoded token transfer information from input data
 */
export interface DecodedTokenTransfer {
  to: string;
  amount: string;
  methodName: string;
}

/**
 * Decodes ERC20 transfer input data
 * ERC20 transfer method signature: 0xa9059cbb
 * Format: 0xa9059cbb + 32 bytes (to address, padded) + 32 bytes (amount)
 * @param inputData - The transaction input data
 * @returns Decoded transfer info or null if not a transfer
 */
export function decodeTokenTransferInput(inputData: string | undefined | null): DecodedTokenTransfer | null {
  if (!inputData || inputData === '0x' || inputData.length < 10) {
    return null;
  }

  // Normalize input
  const data = inputData.toLowerCase();

  // Check for ERC20 transfer method signature (0xa9059cbb)
  if (data.startsWith('0xa9059cbb')) {
    // transfer(address,uint256)
    // Expected length: 0x (2) + method (8) + address (64) + amount (64) = 138
    if (data.length !== 138) {
      return null;
    }

    try {
      // Extract recipient address (bytes 10-74, last 40 chars are the address)
      const toAddressPadded = data.slice(10, 74);
      const toAddress = 'Z' + toAddressPadded.slice(-40);

      // Extract amount (bytes 74-138)
      const amountHex = '0x' + data.slice(74);
      const amount = BigInt(amountHex).toString();

      return {
        to: toAddress,
        amount: amount,
        methodName: 'transfer'
      };
    } catch (error) {
      console.error('Error decoding transfer input:', error);
      return null;
    }
  }

  // Check for ERC20 transferFrom method signature (0x23b872dd)
  if (data.startsWith('0x23b872dd')) {
    // transferFrom(address,address,uint256)
    // Expected length: 0x (2) + method (8) + from (64) + to (64) + amount (64) = 202
    if (data.length !== 202) {
      return null;
    }

    try {
      // Extract from address (bytes 10-74)
      const fromAddressPadded = data.slice(10, 74);
      const fromAddress = 'Z' + fromAddressPadded.slice(-40);

      // Extract to address (bytes 74-138)
      const toAddressPadded = data.slice(74, 138);
      const toAddress = 'Z' + toAddressPadded.slice(-40);

      // Extract amount (bytes 138-202)
      const amountHex = '0x' + data.slice(138);
      const amount = BigInt(amountHex).toString();

      return {
        to: toAddress,
        amount: amount,
        methodName: 'transferFrom'
      };
    } catch (error) {
      console.error('Error decoding transferFrom input:', error);
      return null;
    }
  }

  return null;
}

/**
 * Formats a token amount with proper decimals
 * @param amount - The raw token amount (as string)
 * @param decimals - The number of decimals for the token
 * @returns Formatted amount string
 */
export function formatTokenAmount(amount: string | undefined | null, decimals: number = 18): string {
  if (!amount || amount === '0' || amount === '0x0') {
    return '0';
  }

  try {
    // Handle hex strings
    let rawAmount: bigint;
    if (amount.startsWith('0x')) {
      rawAmount = BigInt(amount);
    } else {
      rawAmount = BigInt(amount);
    }

    if (rawAmount === BigInt(0)) {
      return '0';
    }

    // Convert to decimal representation
    const divisor = BigInt(10 ** decimals);
    const wholePart = rawAmount / divisor;
    const fractionalPart = rawAmount % divisor;

    // Format the fractional part with leading zeros
    let fractionalStr = fractionalPart.toString().padStart(decimals, '0');
    // Remove trailing zeros
    fractionalStr = fractionalStr.replace(/0+$/, '');

    if (fractionalStr === '') {
      // Format whole part with thousand separators
      return wholePart.toLocaleString('en-US');
    }

    // Format whole part with thousand separators and add fractional part
    return `${wholePart.toLocaleString('en-US')}.${fractionalStr}`;
  } catch (error) {
    console.error('Error formatting token amount:', error, amount);
    return amount;
  }
}
