'use client';

/* Follows the shared badge recipe (see Badge.tsx): status fill at /10,
   border at /25, status-colored text. */
const styles: Record<string, string> = {
  finalized: 'bg-success/10 text-success border-success/25',
  justified: 'bg-info/10 text-info border-info/25',
  pending: 'bg-warning/10 text-warning border-warning/25',
  proposed: 'bg-success/10 text-success border-success/25',
  missed: 'bg-error/10 text-error border-error/25',
};

export default function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-medium border ${styles[status] || styles.pending}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}
