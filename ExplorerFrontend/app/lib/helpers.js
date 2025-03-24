'use client';

export function decodeToHex(input, format) {
  const decoded = atob(input);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export function toFixed(x) {
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
    var e = parseInt(num.toString().split('e-')[1]);
    if (e) {
        let val = num * Math.pow(10, e-1);
        return '0.' + (new Array(e)).join('0') + val.toString().substring(2);
    }
  } else {
    var e = parseInt(num.toString().split('+')[1]);
    if (e > 20) {
        e -= 20;
        let val = num / Math.pow(10, e);
        return val + (new Array(e+1)).join('0');
    }
  }
  return num.toString();
}

export function formatGas(amount) {
  // Handle undefined or null
  if (amount === undefined || amount === null) {
    return ['0', 'Shor'];
  }

  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0', 'Shor'];
  }

  try {
    let value;
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

export function formatAmount(amount) {
  // Handle undefined or null
  if (amount === undefined || amount === null) {
    return ['0.00', 'QRL'];
  }

  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0.00', 'QRL'];
  }

  let totalNum;
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
    else if (typeof amount === 'number' || (typeof amount === 'string' && !isNaN(amount))) {
      const floatValue = parseFloat(amount);
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

export function normalizeHexString(hexData) {
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

export function epochToISO(timestamp) {
  if (!timestamp) return '1970-01-01';
  const date = new Date(timestamp * 1000); 
  const datePart = date.toISOString().split('T')[0];
  return datePart;
}

export function formatTimestamp(timestamp) {
  if (!timestamp) return '';
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

export function formatNumber(value) {
  if (typeof value !== "number" || isNaN(value)) {
    return "Error";
  }
  if (value >= 1e12) {
      value = (value / 1e12).toFixed(2) + 'T';
  } else if (value >= 1e9) {
      value = (value / 1e9).toFixed(2) + 'B';
  } else if (value >= 1e6) {
      value = (value / 1e6).toFixed(2) + 'M';
  } else if (value >= 1e3) {
      value = (value / 1e3).toFixed(2) + 'K';
  } else {
      value = value.toFixed(2);
  }
  return '$' + value;
}

export function formatNumberWithCommas(x) {
  if (x === undefined || x === null) {
    return "0";
  }
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

export function epochsToDays(epochs) {
  // Each epoch is 128 slots
  // Each slot takes 60 seconds
  // So each epoch is 128 * 60 seconds
  // Convert to days
  return (epochs * 128 * 60) / (24 * 60 * 60);
}

export function truncateHash(hash, startLength = 6, endLength = 4) {
  if (!hash || hash.length < startLength + endLength) return hash;
  return `${hash.slice(0, startLength)}...${hash.slice(-endLength)}`;
}

/**
 * Formats an address to ensure it has the correct prefix (Z for QRL addresses, 0x for contract addresses)
 * @param {string} address - The address to format
 * @returns {string} - The formatted address
 */
export function formatAddress(address) {
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
