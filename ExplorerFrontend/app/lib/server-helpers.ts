export function decodeBase64ToHexadecimal(rawData: string): string {
  // For edge runtime, we need to use the Web APIs
  const binaryStr = atob(rawData);
  let hex = '';
  for (let i = 0; i < binaryStr.length; i++) {
    const byte = binaryStr.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return '0x' + hex;
}
