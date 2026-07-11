interface DetailRowProps {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
}

export default function DetailRow({ label, children, mono }: DetailRowProps): JSX.Element {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start py-3 border-b border-border/70 last:border-b-0">
      <span className="text-[13px] text-text-muted sm:w-44 flex-shrink-0 mb-1 sm:mb-0 sm:pt-px">{label}</span>
      <div className={`text-sm text-text-primary min-w-0 ${mono ? 'font-mono break-all' : ''}`}>
        {children}
      </div>
    </div>
  );
}
