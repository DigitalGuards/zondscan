'use client';

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPrevious: () => void;
  onNext: () => void;
}

export default function Pagination({ currentPage, totalPages, onPrevious, onNext }: PaginationProps): JSX.Element {
  return (
    // Landmark wrapper so screen readers list pagination controls under
    // "Pagination" in their landmark menu instead of generic regions.
    <nav aria-label="Pagination" className="flex justify-center items-center gap-3 text-sm text-text-secondary mt-6">
      <button
        onClick={onPrevious}
        disabled={currentPage <= 1}
        aria-label="Go to previous page"
        className="btn-secondary text-sm"
      >
        Previous
      </button>
      {/* aria-live so SR users hear the page transition announced when
          the parent component re-renders with a new currentPage. */}
      <span className="num text-xs sm:text-sm px-2" aria-live="polite" aria-atomic="true">
        Page {currentPage} of {totalPages}
      </span>
      <button
        onClick={onNext}
        disabled={currentPage >= totalPages}
        aria-label="Go to next page"
        className="btn-secondary text-sm"
      >
        Next
      </button>
    </nav>
  );
}
