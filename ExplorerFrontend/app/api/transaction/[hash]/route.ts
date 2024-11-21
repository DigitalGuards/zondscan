import { NextResponse } from 'next/server';
import config from '../../../../config';
import { decodeBase64ToHexadecimal } from '../../../lib/helpers';

export async function GET(
  request: Request,
  { params }: { params: { hash: string } }
) {
  try {
    const response = await fetch(`${config.handlerUrl}/tx/${params.hash}`);
    
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
