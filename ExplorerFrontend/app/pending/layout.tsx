export default function PendingLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="min-h-screen bg-[#1a1a1a]">
      {children}
    </div>
  );
}
