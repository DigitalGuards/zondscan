export function decodeToHex(input: string, format: BufferEncoding): string {
  const buffer = Buffer.from(input, format);
  return buffer.toString('hex');
}

export function toFixed(x: number): string {
  if (Math.abs(x) < 1.0) {
    let e = parseInt(x.toString().split('e-')[1]);
    if (e) {
      x *= Math.pow(10, e-1);
      const zeros = new Array(e).join('0');
      return '0.' + zeros + x.toString().substring(2);
    }
  } else {
    let e = parseInt(x.toString().split('+')[1]);
    if (e > 20) {
      e -= 20;
      x /= Math.pow(10, e);
      x = Number(x.toString() + (new Array(e+1)).join('0'));
    }
  }
  return x.toString();
}

export function decodeBase64ToHexadecimal(rawData: string): string {
  const buffer = Buffer.from(rawData, 'base64');
  return buffer.toString('hex');
}

export function epochToISO(timestamp: number): string {
  const date = new Date(timestamp * 1000); 
  return date.toISOString().split('T')[0];
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

export function formatNumberWithCommas(x: string | number): string {
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}
