"use client";

export default function TransactionsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="transactions-layout">
      {children}
    </div>
  );
}
