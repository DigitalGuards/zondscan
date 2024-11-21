import { Suspense } from 'react';
import { notFound } from 'next/navigation';
import BlocksClient from './blocks-client';
import { BlocksResponse } from './types';

async function getBlocks(page: string): Promise<BlocksResponse> {
  const handlerUrl = process.env.NEXT_PUBLIC_HANDLER_URL || 'http://localhost:8080';
  
  try {
    const response = await fetch(`${handlerUrl}/blocks?page=${page}`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json'
      },
      next: {
        revalidate: 0
      }
    });

    if (!response.ok) {
      if (response.status === 404) {
        notFound();
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return response.json();
  } catch (error) {
    console.error('Error fetching blocks:', error);
    throw error;
  }
}

function LoadingUI() {
  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
      <div className="space-y-4">
        {[...Array(5)].map((_, i) => (
          <div 
            key={i}
            className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] p-6 animate-pulse"
          >
            <div className="flex flex-col md:flex-row items-center">
              <div className="w-48 flex flex-col items-center">
                <div className="w-8 h-8 bg-gray-700 rounded-lg mb-2"></div>
                <div className="h-4 w-20 bg-gray-700 rounded"></div>
              </div>
              <div className="flex-1 md:ml-8 space-y-2">
                <div className="h-6 w-32 bg-gray-700 rounded"></div>
                <div className="h-4 w-full bg-gray-700 rounded"></div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default async function BlocksPage({ params }: { params: { query: string } }) {
  const resolvedParams = await Promise.resolve(params);
  const pageNumber = resolvedParams?.query || '1';

  try {
    const data = await getBlocks(pageNumber);

    return (
      <main>
        <h1 className="sr-only">Blocks - Page {pageNumber}</h1>
        <Suspense fallback={<LoadingUI />}>
          <BlocksClient 
            initialData={data}
            initialPage={pageNumber} 
          />
        </Suspense>
      </main>
    );
  } catch (error) {
    return (
      <div role="alert" className="p-8">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Error</h1>
        <p className="text-gray-300">Failed to load blocks. Please try again later.</p>
        {process.env.NODE_ENV === 'development' && (
          <pre className="mt-2 text-sm text-red-500">
            {error instanceof Error ? error.message : 'Unknown error'}
          </pre>
        )}
      </div>
    );
  }
}

// Generate static params for common page numbers
export async function generateStaticParams() {
  // Pre-render the first 10 pages
  return Array.from({ length: 10 }, (_, i) => ({
    query: String(i + 1),
  }));
}
