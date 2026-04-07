'use client';

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPrevious: () => void;
  onNext: () => void;
}

export default function Pagination({ currentPage, totalPages, onPrevious, onNext }: PaginationProps): JSX.Element {
  return (
    <div className="flex justify-center items-center gap-3 text-sm text-gray-300 mt-6">
      <button
        onClick={onPrevious}
        disabled={currentPage <= 1}
        aria-label="Go to previous page"
        className="px-4 py-2 rounded-lg bg-[#1e1e1e] text-gray-300 border border-[#2a2a2a]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#2a2a2a]
                   transition-colors"
      >
        Previous
      </button>
      <span className="tabular-nums">Page {currentPage} of {totalPages}</span>
      <button
        onClick={onNext}
        disabled={currentPage >= totalPages}
        aria-label="Go to next page"
        className="px-4 py-2 rounded-lg bg-[#1e1e1e] text-gray-300 border border-[#2a2a2a]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#2a2a2a]
                   transition-colors"
      >
        Next
      </button>
    </div>
  );
}
