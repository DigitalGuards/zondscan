'use client';

export function decodeToHex(input, format) {
  // Using browser's built-in atob for base64 decoding
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
  if (amount === 0 || amount === '0') {
    return ['0', 'QRL'];
  }

  // Convert string to number if needed
  const num = typeof amount === 'string' ? parseFloat(amount) : amount;

  // Handle NaN
  if (isNaN(num)) {
    return ['0', 'QRL'];
  }

  // Convert to shor (1 QRL = 1e9 shor)
  const shorValue = num * 1e9;

  // If it's >= 1 QRL, show in QRL
  if (num >= 1) {
    return [num.toString(), 'QRL'];
  }

  // If it's less than 1 shor or 0, show "0 shor"
  if (shorValue < 1) {
    return ['0', 'shor'];
  }

  // Otherwise show the exact number of shor
  return [Math.floor(shorValue).toString(), 'shor'];
}

export function decodeBase64ToHexadecimal(rawData) {
  // Using browser's built-in atob for base64 decoding
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
    console.error("Invalid value passed to formatNumber:", value);
    return "Error";  // Default value
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
