'use client';

const styles: Record<string, string> = {
  finalized: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
  justified: 'bg-blue-500/15 text-blue-400 border-blue-500/20',
  pending: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/20',
  proposed: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
  missed: 'bg-red-500/15 text-red-400 border-red-500/20',
};

export default function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium border ${styles[status] || styles.pending}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}
