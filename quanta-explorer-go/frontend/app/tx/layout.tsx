"use client";

export default function TransactionLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="transaction-layout">
      {children}
    </div>
  );
}
