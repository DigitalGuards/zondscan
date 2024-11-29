'use client';

export function decodeToHex(input: string, format?: string): string {
  const decoded = atob(input);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export function toFixed(x: string | number | undefined | null): string {
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
    const exponent = parseInt(num.toString().split('e-')[1]);
    if (exponent) {
        const val = num * Math.pow(10, exponent-1);
        return '0.' + (new Array(exponent)).join('0') + val.toString().substring(2);
    }
  } else {
    const exponent = parseInt(num.toString().split('+')[1]);
    if (exponent > 20) {
        const adjustedExp = exponent - 20;
        const val = num / Math.pow(10, adjustedExp);
        return val + (new Array(adjustedExp+1)).join('0');
    }
  }
  return num.toString();
}

export function formatGas(amount: string | number | bigint): [string, string] {
  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0', ''];
  }

  try {
    // Handle hex strings (e.g., "0x123")
    if (typeof amount === 'string' && amount.startsWith('0x')) {
      const value = parseInt(amount, 16);
      return [value.toString(), ''];
    }
    // Handle number values
    else if (typeof amount === 'number') {
      return [amount.toString(), ''];
    }
    // Handle other formats
    else {
      const value = BigInt(amount);
      return [value.toString(), ''];
    }
  } catch (error) {
    console.error('Error converting gas amount:', error, amount);
    return ['0', ''];
  }
}

export function formatAmount(amount: string | number | bigint): [string, string] {
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
    // Handle float values (already scaled) from database
    else if (typeof amount === 'number' && amount < 1000000000000000000) {
      totalNum = amount;
    }
    // Handle raw number values (need scaling) from database
    else if (typeof amount === 'number') {
      const value = BigInt(Math.floor(amount));
      const divisor = BigInt('1000000000000000000'); // 10^18
      const wholePart = value / divisor;
      const fractionalPart = value % divisor;
      totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
    }
    // Handle other formats
    else {
      const value = BigInt(amount);
      const divisor = BigInt('1000000000000000000'); // 10^18
      const wholePart = value / divisor;
      const fractionalPart = value % divisor;
      totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
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
    const str = totalNum.toFixed(21).replace(/\.?0+$/, '');
    // Remove any scientific notation
    const parts = str.split('e');
    if (parts.length === 2) {
      const mantissa = parts[0];
      const exponent = parseInt(parts[1]);
      if (exponent < 0) {
        const abs = Math.abs(exponent);
        return ['0.' + '0'.repeat(abs-1) + mantissa.replace('.', ''), 'QRL'];
      }
    }
    return [str, 'QRL'];
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

export function decodeBase64ToHexadecimal(rawData: string): string {
  const decoded = atob(rawData);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export function epochToISO(timestamp: number): string {
  const date = new Date(timestamp * 1000); 
  const datePart = date.toISOString().split('T')[0];
  return datePart;
}

export function formatTimestamp(timestamp: number): string {
  const date = new Date(timestamp * 1000);
  return date.toLocaleString('en-US', {
    day: 'numeric',
    month: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
    second: 'numeric'
  });
}

export function formatNumber(value: number): string {
  if (typeof value !== "number" || isNaN(value)) {
    return "Error";
  }
  let formattedValue: string;
  if (value >= 1e12) {
      formattedValue = (value / 1e12).toFixed(2) + 'T';
  } else if (value >= 1e9) {
      formattedValue = (value / 1e9).toFixed(2) + 'B';
  } else if (value >= 1e6) {
      formattedValue = (value / 1e6).toFixed(2) + 'M';
  } else if (value >= 1e3) {
      formattedValue = (value / 1e3).toFixed(2) + 'K';
  } else {
      formattedValue = value.toFixed(2);
  }
  return '$' + formattedValue;
}

export function formatNumberWithCommas(x: number | string | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}
