interface DetailRowProps {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
}

export default function DetailRow({ label, children, mono }: DetailRowProps): JSX.Element {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start py-3 border-b border-[#3d3d3d]/30 last:border-b-0">
      <span className="text-sm text-gray-500 sm:w-44 flex-shrink-0 mb-1 sm:mb-0">{label}</span>
      <div className={`text-sm text-gray-200 min-w-0 ${mono ? 'font-mono break-all' : ''}`}>
        {children}
      </div>
    </div>
  );
}
