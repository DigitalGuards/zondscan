"use client";

import { useState, useEffect } from "react";
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getFilteredRowModel,
  getCoreRowModel,
  getPaginationRowModel,
  ColumnDef
} from "@tanstack/react-table";
import { decodeBase64ToHexadecimal, epochToISO, toFixed, formatAmount } from "../lib/helpers";
import DebouncedInput from "./DebouncedInput";
import { DownloadIcon } from "./Icons";
import { DownloadBtn, DownloadBtnInternal } from "./DownloadBtn";
import Link from "next/link";
import { Transaction, InternalTransaction } from "./types";

const truncateMiddle = (str: string, startChars = 8, endChars = 8): string => {
  if (str.length <= startChars + endChars) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
};

interface TableProps {
  transactions: Transaction[];
  internalt: InternalTransaction[];
}

type TableData = {
  transactions: Transaction[];
  internalTransactions: InternalTransaction[];
};

export default function TanStackTable({ transactions, internalt }: TableProps) {
  const [windowWidth, setWindowWidth] = useState(typeof window !== 'undefined' ? window.innerWidth : 0);
  const [globalFilter, setGlobalFilter] = useState("");
  const [showInternal, setShowInternal] = useState(false);

  useEffect(() => {
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const columnHelper = createColumnHelper<Transaction>();
  const internalColumnHelper = createColumnHelper<InternalTransaction>();
  const inOutMap = ["Out", "In"];
  const txTypeMap = ["Coinbase", "Attest", "Transfer", "Stake"];

  const columns: ColumnDef<Transaction, any>[] = [
    columnHelper.accessor(() => "", {
      id: "Number",
      cell: (info) => <span>{transactions.length - info.row.index}</span>,
      header: "Number",
    }),
    columnHelper.accessor("InOut", {
      cell: (info) => <span>{inOutMap[info.getValue()]}</span>,
      header: "In/Out",
    }),
    columnHelper.accessor("TxType", {
      cell: (info) => <span>{txTypeMap[info.getValue()]}</span>,
      header: "Transaction Type",
    }),
    columnHelper.accessor((row) => ({ from: row.From, to: row.To }), {
      id: "Addresses",
      cell: (info) => {
        const { from, to } = info.getValue();
        const fromAddress = from ? "0x" + decodeBase64ToHexadecimal(from) : "";
        const toAddress = to ? "0x" + decodeBase64ToHexadecimal(to) : "";
        return (
          <div className="flex flex-col gap-1">
            {fromAddress && (
              <div className="flex items-center gap-1">
                <span className="text-gray-400 text-sm">From:</span>
                <Link href={"/address/" + fromAddress} title={fromAddress}>
                  {truncateMiddle(fromAddress)}
                </Link>
              </div>
            )}
            {toAddress && (
              <div className="flex items-center gap-1">
                <span className="text-gray-400 text-sm">To:</span>
                <Link href={"/address/" + toAddress} title={toAddress}>
                  {truncateMiddle(toAddress)}
                </Link>
              </div>
            )}
          </div>
        );
      },
      header: "From/To",
    }),
    columnHelper.accessor("TxHash", {
      cell: (info) => {
        const fullHash = "0x" + decodeBase64ToHexadecimal(info.getValue());
        return (
          <span>
            <Link href={"/tx/" + fullHash} title={fullHash}>
              {truncateMiddle(fullHash)}
            </Link>
          </span>
        );
      },
      header: "Transaction Hash",
    }),
    columnHelper.accessor("TimeStamp", {
      cell: (info) => <span>{epochToISO(info.getValue())}</span>,
      header: "Timestamp",
    }),
    columnHelper.accessor((row) => {
      const [amount] = formatAmount(row.Amount);
      return amount;
    }, {
      id: "Amount",
      header: "Amount (QRL)",
    }),
    columnHelper.accessor((row) => {
      const [fees] = formatAmount(row.Paidfees);
      return fees;
    }, {
      id: "PaidFees",
      header: "Paid Fees (QRL)",
    }),
  ];

  const internalColumns: ColumnDef<InternalTransaction, any>[] = [
    internalColumnHelper.accessor(() => "", {
      id: "Number",
      cell: (info) => <span>{internalt.length - info.row.index}</span>,
      header: "Number",
    }),
    internalColumnHelper.accessor("Type", {
      cell: (info) => <span>{atob(String(info.getValue()))}</span>,
      header: "Type",
    }),
    internalColumnHelper.accessor("From", {
      cell: (info) => {
        const fullAddress = "0x" + decodeBase64ToHexadecimal(info.getValue());
        return (
          <span>
            <Link href={"/address/" + fullAddress} title={fullAddress}>
              {truncateMiddle(fullAddress)}
            </Link>
          </span>
        );
      },
      header: "From",
    }),
    internalColumnHelper.accessor("To", {
      cell: (info) => {
        const fullAddress = "0x" + decodeBase64ToHexadecimal(info.getValue());
        return (
          <span>
            <Link href={"/address/" + fullAddress} title={fullAddress}>
              {truncateMiddle(fullAddress)}
            </Link>
          </span>
        );
      },
      header: "To",
    }),
    internalColumnHelper.accessor("Hash", {
      cell: (info) => {
        const fullHash = "0x" + decodeBase64ToHexadecimal(info.getValue());
        return (
          <span>
            <Link href={"/tx/" + fullHash} title={fullHash}>
              {truncateMiddle(fullHash)}
            </Link>
          </span>
        );
      },
      header: "Transaction Hash",
    }),
    internalColumnHelper.accessor((row) => toFixed(row.Value), {
      id: "Value",
      header: "Value (QRL)",
    }),
    internalColumnHelper.accessor((row) => toFixed(row.GasUsed), {
      id: "GasUsed",
      header: "Gas Used (in Units)",
    }),
    internalColumnHelper.accessor((row) => toFixed(row.AmountFunctionIdentifier), {
      id: "AmountFunctionIdentifier",
      header: "Token Units",
    }),
    internalColumnHelper.accessor("BlockTimestamp", {
      cell: (info) => <span>{epochToISO(info.getValue())}</span>,
      header: "Timestamp",
    }),
    internalColumnHelper.accessor("Output", {
      cell: (info) => <span>{info.getValue() === 1 ? "Success" : "Failure"}</span>,
      header: "Status",
    }),
  ];

  const table = useReactTable({
    data: showInternal ? internalt : transactions,
    columns: showInternal ? internalColumns : columns,
    state: {
      globalFilter,
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const renderMobileCard = (row: any) => {
    const data = row.original;
    const isInternal = showInternal;
    
    return (
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-gray-400">Transaction Type</div>
              <div className="text-sm text-white">
                {isInternal ? data.Type : txTypeMap[data.TxType]}
              </div>
            </div>
            {!isInternal && (
              <div className="px-2 py-1 rounded bg-[#3d3d3d] bg-opacity-40">
                <span className="text-xs text-[#ffa729]">{inOutMap[data.InOut]}</span>
              </div>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <Link 
              href={"/tx/0x" + decodeBase64ToHexadecimal(data.TxHash)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + decodeBase64ToHexadecimal(data.TxHash)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            {data.From && (
              <Link 
                href={"/address/0x" + decodeBase64ToHexadecimal(data.From)}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {"0x" + decodeBase64ToHexadecimal(data.From)}
              </Link>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            {data.To && (
              <Link 
                href={"/address/0x" + decodeBase64ToHexadecimal(data.To)}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {"0x" + decodeBase64ToHexadecimal(data.To)}
              </Link>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-gray-400">Amount</div>
              <div className="text-sm text-white">
                {(() => {
                  const [amount, unit] = formatAmount(data.Amount || 0);
                  return <>{amount} {unit}</>;
                })()}
              </div>
            </div>
            {!isInternal && data.Paidfees !== undefined && (
              <div>
                <div className="text-xs text-gray-400">Fees</div>
                <div className="text-sm text-white">
                  {(() => {
                    const [fees, unit] = formatAmount(data.Paidfees);
                    return <>{fees} {unit}</>;
                  })()}
                </div>
              </div>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{epochToISO(data.TimeStamp)}</div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="w-full">
      <div className="p-4 border-b border-[#3d3d3d] space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-center space-x-4">
            <button
              onClick={() => setShowInternal(false)}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                !showInternal
                  ? "bg-[#ffa729] text-black"
                  : "text-gray-400 hover:text-white"
              }`}
            >
              Transactions
            </button>
            <button
              onClick={() => setShowInternal(true)}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                showInternal
                  ? "bg-[#ffa729] text-black"
                  : "text-gray-400 hover:text-white"
              }`}
            >
              Internal Txns
            </button>
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={globalFilter ?? ""}
              onChange={(value) => setGlobalFilter(String(value))}
              className="px-4 py-2 text-sm bg-[#1a1a1a] border border-[#3d3d3d] rounded-lg focus:outline-none focus:border-[#ffa729] text-white w-full md:w-auto"
              placeholder="Search transactions..."
            />
            {showInternal ? (
              <DownloadBtnInternal data={internalt} />
            ) : (
              <DownloadBtn data={transactions} />
            )}
          </div>
        </div>
      </div>

      {windowWidth < 768 ? (
        <div className="divide-y divide-[#3d3d3d]">
          {table.getRowModel().rows.map((row) => renderMobileCard(row))}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              {table.getHeaderGroups().map((headerGroup) => (
                <tr key={headerGroup.id} className="border-b border-[#3d3d3d]">
                  {headerGroup.headers.map((header) => (
                    <th
                      key={header.id}
                      className="px-4 py-3 text-left text-sm font-medium text-[#ffa729]"
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext()
                          )}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)]"
                >
                  {row.getVisibleCells().map((cell) => (
                    <td
                      key={cell.id}
                      className="px-4 py-3 text-sm text-gray-300"
                    >
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="p-4 border-t border-[#3d3d3d]">
        <div className="flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <button
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => table.setPageIndex(0)}
              disabled={!table.getCanPreviousPage()}
            >
              {"<<"}
            </button>
            <button
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              Previous
            </button>
            <button
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              Next
            </button>
            <button
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => table.setPageIndex(table.getPageCount() - 1)}
              disabled={!table.getCanNextPage()}
            >
              {">>"}
            </button>
          </div>
          <div className="text-sm text-gray-400">
            Page {table.getState().pagination.pageIndex + 1} of{" "}
            {table.getPageCount()}
          </div>
        </div>
      </div>
    </div>
  );
}
