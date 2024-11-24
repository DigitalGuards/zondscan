export function decodeToHex(input, format) {
  const buffer = Buffer.from(input, format);
  return buffer.toString('hex');
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

export function decodeBase64ToHexadecimal(rawData) {
  const buffer = Buffer.from(rawData, 'base64');
  const bufString = buffer.toString('hex');
  return bufString
}

export function epochToISO(timestamp) {
  const date = new Date(timestamp * 1000); 
  const datePart = date.toISOString().split('T')[0];
  return datePart
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
