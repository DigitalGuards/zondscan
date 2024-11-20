"use client";

export default function ContractsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="card m-4">
      {children}
    </div>
  );
}
