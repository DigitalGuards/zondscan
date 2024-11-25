import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';

export const runtime = 'edge';

interface GenerateRequest {
  address: string;
}

function generateRandomHex(length: number): string {
  const array = new Uint8Array(length);
  crypto.getRandomValues(array);
  return Array.from(array)
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}

export async function POST(request: NextRequest): Promise<NextResponse> {
  try {
    const body = await request.json() as GenerateRequest;
    
    if (!body.address) {
      return NextResponse.json(
        { error: 'Address is required' },
        { status: 400 }
      );
    }
    
    const randomMessage = generateRandomHex(32); // 32 bytes = 64 hex chars

    return NextResponse.json({ challenge: randomMessage });
  } catch (error) {
    console.error('Error generating challenge:', error);
    return NextResponse.json(
      { error: 'Failed to generate challenge' },
      { status: 500 }
    );
  }
}
