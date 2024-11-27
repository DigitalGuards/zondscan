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
    const e = parseInt(num.toString().split('e-')[1]);
    if (e) {
      const val = num * Math.pow(10, e-1);
      return '0.' + (new Array(e)).join('0') + val.toString().substring(2);
    }
  } else {
    const e = parseInt(num.toString().split('+')[1]);
    if (e > 20) {
      const val = num / Math.pow(10, e-20);
      return val + (new Array(e+1)).join('0');
    }
  }
  return num.toString();
}

export function formatAmount(amount: string | number): [string, string] {
  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0', 'QRL'];
  }

  let value: bigint;
  try {
    // Handle hex strings (e.g., "0x123")
    if (typeof amount === 'string' && amount.startsWith('0x')) {
      value = BigInt(amount);
    }
    // Handle regular strings or numbers
    else {
      value = BigInt(typeof amount === 'string' ? amount : Math.floor(amount));
    }
  } catch (error) {
    console.error('Error converting amount to BigInt:', error);
    return ['0', 'QRL'];
  }

  // Convert to QRL (1 QRL = 10^18 shor)
  const qrl = value / BigInt(1e18);
  const remainingShor = value % BigInt(1e18);

  // If there's a QRL amount, show it with up to 8 decimal places
  if (qrl > BigInt(0)) {
    const shorFraction = Number(remainingShor) / 1e18;
    const total = Number(qrl) + shorFraction;
    return [total.toFixed(8).replace(/\.?0+$/, ''), 'QRL'];
  }

  // If it's just shor, show the shor amount
  if (remainingShor === BigInt(0)) {
    return ['0', 'shor'];
  }

  return [remainingShor.toString(), 'shor'];
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
    console.error("Invalid value passed to formatNumber:", value);
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

export function formatNumberWithCommas(x: string | number | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}
