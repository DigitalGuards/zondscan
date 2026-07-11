'use client';

interface VerifiedBadgeProps {
  /** Compact variant fits inside table rows / headers; default sits inline next to titles. */
  size?: 'sm' | 'md';
  /** Tooltip / aria text. Defaults to "Source verified". */
  title?: string;
}

/**
 * Small green pill rendered next to verified-contract titles. Visual only ,
 * the parent must already have determined `contractData.verified === true`
 * before mounting this.
 */
export default function VerifiedBadge({ size = 'md', title = 'Source verified' }: VerifiedBadgeProps): JSX.Element {
  const sizing = size === 'sm' ? 'px-1.5 py-0.5 text-[10px]' : 'px-2 py-0.5 text-xs';
  return (
    <span
      title={title}
      aria-label={title}
      className={`inline-flex items-center gap-1 rounded-full border border-green-500/40 bg-green-500/10 text-success font-medium ${sizing}`}
    >
      <svg
        className={size === 'sm' ? 'h-2.5 w-2.5' : 'h-3 w-3'}
        viewBox="0 0 20 20"
        fill="currentColor"
        aria-hidden="true"
      >
        <path
          fillRule="evenodd"
          d="M10 18a8 8 0 1 0 0-16 8 8 0 0 0 0 16Zm3.707-9.293a1 1 0 1 0-1.414-1.414L9 10.586 7.707 9.293a1 1 0 0 0-1.414 1.414l2 2a1 1 0 0 0 1.414 0l4-4Z"
          clipRule="evenodd"
        />
      </svg>
      Verified
    </span>
  );
}
