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

export function formatAmount(amount) {
  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0', 'QRL'];
  }

  let value;
  try {
    if (typeof amount === 'string') {
      if (amount.startsWith('0x')) {
        // Handle hex strings
        value = BigInt(amount);
      } else {
        // Handle decimal strings
        value = BigInt(amount);
      }
    } else if (typeof amount === 'number') {
      // Handle scientific notation and regular numbers
      if (amount.toString().includes('e')) {
        // Convert scientific notation to full decimal string
        const str = amount.toFixed(20);
        // Remove trailing zeros after decimal point
        const cleanStr = str.replace(/\.?0+$/, '');
        // Convert to shor (multiply by 1e18)
        const shorAmount = parseFloat(cleanStr) * 1e18;
        value = BigInt(Math.round(shorAmount));
      } else {
        // Regular number - convert to shor
        const shorAmount = amount * 1e18;
        value = BigInt(Math.round(shorAmount));
      }
    } else {
      throw new Error('Unsupported amount type');
    }
  } catch (error) {
    return ['0', 'QRL'];
  }

  // Now value is in shor (smallest unit, 18 decimal places)
  // If it's less than 1 QRL (1e18 shor), show in shor
  if (value < BigInt(1e18)) {
    if (value === BigInt(0)) {
      return ['0', 'shor'];
    }
    return [value.toString(), 'shor'];
  }

  // Convert to QRL (divide by 1e18) and show with up to 8 decimal places
  const qrlValue = Number(value) / 1e18;
  return [qrlValue.toFixed(8).replace(/\.?0+$/, ''), 'QRL'];
}

export function decodeBase64ToHexadecimal(rawData) {
  const decoded = atob(rawData);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export function epochToISO(timestamp) {
  const date = new Date(timestamp * 1000); 
  const datePart = date.toISOString().split('T')[0];
  return datePart;
}

export function formatTimestamp(timestamp) {
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
