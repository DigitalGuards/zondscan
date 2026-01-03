"use client";

import type { ReactNode } from "react";

export default function AddressLayout({
  children,
}: {
  children: ReactNode;
}): JSX.Element {
  return (
    <div className="page-container">
      <div className="page-content">
        {children}
      </div>
    </div>
  );
}
