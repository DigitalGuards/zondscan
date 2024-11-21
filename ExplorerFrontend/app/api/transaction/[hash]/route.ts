import { NextResponse } from 'next/server';
import config from '../../../../config';
import { decodeBase64ToHexadecimal } from '../../../lib/helpers';

export async function GET(
  request: Request,
  { params }: { params: { hash: string } }
) {
  try {
    // Get the params object first
    const resolvedParams = await Promise.resolve(params);
    
    // Then destructure the hash after awaiting
    const { hash } = resolvedParams;

    if (!hash) {
      return NextResponse.json(
        { error: 'Transaction hash is required' },
        { status: 400 }
      );
    }

    const response = await fetch(`${config.handlerUrl}/tx/${hash}`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json'
      },
      cache: 'no-store'
    });
    
    if (!response.ok) {
      throw new Error('Failed to fetch transaction');
    }

    const data = await response.json();
    
    // Transform the addresses from base64 to hex
    if (data.response) {
      if (data.response.from) {
        data.response.from = "0x" + decodeBase64ToHexadecimal(data.response.from);
      }
      if (data.response.to) {
        data.response.to = "0x" + decodeBase64ToHexadecimal(data.response.to);
      }
    }

    return NextResponse.json(data);
  } catch (error) {
    console.error('Error fetching transaction:', error);
    return NextResponse.json(
      { error: 'Failed to fetch transaction details' },
      { status: 500 }
    );
  }
}
