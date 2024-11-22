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

export default async function BlocksPage({ params }: { params: { query: string } }) {
  const resolvedParams = await Promise.resolve(params);
  const pageNumber = resolvedParams?.query || '1';

  try {
    const data = await getBlocks(pageNumber);

    return (
      <main>
        <h1 className="sr-only">Blocks - Page {pageNumber}</h1>
        <BlocksClient 
          initialData={data}
          initialPage={pageNumber} 
        />
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
