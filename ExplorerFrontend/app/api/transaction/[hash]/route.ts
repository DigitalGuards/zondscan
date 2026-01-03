import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import config from '../../../../config';

export const runtime = 'edge';

interface TransactionResponse {
  response?: {
    from?: string;
    to?: string;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

function decodeBase64ToHex(base64: string): string {
  // For edge runtime, we need to use the Web APIs
  const binaryStr = atob(base64);
  let hex = '';
  for (let i = 0; i < binaryStr.length; i++) {
    const byte = binaryStr.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<Record<string, string>> }
): Promise<NextResponse> {
  try {
    const params = await context.params;
    const hash = params.hash;

    if (!hash) {
      return NextResponse.json(
        { error: 'Transaction hash is required' },
        { status: 400 }
      );
    }

    const response = await fetch(`${config.handlerUrl}/tx/${hash}`, {
      method: 'GET',
      headers: { Accept: 'application/json' },
      next: { revalidate: 60 }, // Cache for 60 seconds
    });

    if (!response.ok) {
      throw new Error('Failed to fetch transaction');
    }

    const data: TransactionResponse = await response.json();

    // Transform the addresses from base64 to hex
    if (data.response) {
      if (data.response.from) {
        data.response.from = '0x' + decodeBase64ToHex(data.response.from);
      }
      if (data.response.to) {
        data.response.to = '0x' + decodeBase64ToHex(data.response.to);
      }
    }

    // Return response with cache headers
    const jsonResponse = NextResponse.json(data);
    jsonResponse.headers.set('Cache-Control', 'public, s-maxage=60');
    
    return jsonResponse;
  } catch (error) {
    console.error('Error fetching transaction:', error);
    return NextResponse.json(
      { error: 'Failed to fetch transaction details' },
      { status: 500 }
    );
  }
}
