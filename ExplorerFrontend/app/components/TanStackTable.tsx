"use client";

import { useState, useEffect, useMemo } from "react";
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getFilteredRowModel,
  getCoreRowModel,
  getPaginationRowModel,
  ColumnDef,
  Row,
  Table,
  HeaderGroup,
  Header,
  Cell
} from "@tanstack/react-table";
import { decodeBase64ToHexadecimal, epochToISO, formatAmount } from "../lib/helpers";
import DebouncedInput from "./DebouncedInput";
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

type TableInstance<T> = Table<T>;

const renderTableHeader = <T extends Transaction | InternalTransaction>(
  table: TableInstance<T>
) => {
  return table.getHeaderGroups().map((headerGroup: HeaderGroup<T>) => (
    <tr key={headerGroup.id} className="border-b border-[#3d3d3d]">
      {headerGroup.headers.map((header: Header<T, unknown>) => (
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
  ));
};

const renderTableBody = <T extends Transaction | InternalTransaction>(
  table: TableInstance<T>
) => {
  return table.getRowModel().rows.map((row: Row<T>) => (
    <tr
      key={row.id}
      className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)]"
    >
      {row.getVisibleCells().map((cell: Cell<T, unknown>) => (
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
  ));
};

const calculateFees = (tx: Transaction): number => {
  // First check if PaidFees is available (decimal format)
  if (typeof tx.PaidFees === 'number') {
    return tx.PaidFees;
  }
  
  // Calculate from gas values (numeric format)
  if (typeof tx.gasUsed !== 'number' || typeof tx.gasPrice !== 'number') return 0;
  
  try {
    // Calculate fees using numeric values
    const gasUsed = BigInt(tx.gasUsed);
    const gasPrice = BigInt(tx.gasPrice);
    
    // Convert the result to a number for consistency with PaidFees format
    return Number(gasUsed * gasPrice) / 1e18; // Convert to QRL units
  } catch (error) {
    console.error('Error calculating fees:', error);
    return 0;
  }
};

export default function TanStackTable({ transactions, internalt }: TableProps) {
  const [mounted, setMounted] = useState(false);
  const [windowWidth, setWindowWidth] = useState(0);
  const [globalFilter, setGlobalFilter] = useState("");
  const [showInternal, setShowInternal] = useState(false);

  useEffect(() => {
    setMounted(true);
    setWindowWidth(window.innerWidth);
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Pre-format the transaction data
  const formattedTransactions = useMemo(() => transactions.map(tx => {
    const [amount, amountUnit] = formatAmount(tx.Amount);
    const fees = calculateFees(tx);
    const [feesFormatted, feesUnit] = formatAmount(fees);
    return {
      ...tx,
      formattedAmount: `${amount} ${amountUnit}`,
      formattedFees: `${feesFormatted} ${feesUnit}`
    };
  }), [transactions]);

  // Pre-format the internal transaction data
  const formattedInternalTransactions = useMemo(() => internalt.map(tx => {
    const [value, valueUnit] = formatAmount(tx.Value);
    return {
      ...tx,
      formattedValue: `${value} ${valueUnit}`
    };
  }), [internalt]);

  const columnHelper = createColumnHelper<any>();
  const internalColumnHelper = createColumnHelper<any>();
  const inOutMap = ["Out", "In"];
  const txTypeMap = ["Coinbase", "Attest", "Transfer", "Stake"];

  const transactionColumns = useMemo(() => [
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
    columnHelper.accessor("formattedAmount", {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Amount",
    }),
    columnHelper.accessor("formattedFees", {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Paid Fees",
    }),
  ], [transactions.length]);

  const internalTransactionColumns = useMemo(() => [
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
    internalColumnHelper.accessor("formattedValue", {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Value",
    }),
    internalColumnHelper.accessor("GasUsed", {
      cell: (info) => <span>{info.getValue()} Units</span>,
      header: "Gas Used (in Units)",
    }),
    internalColumnHelper.accessor("AmountFunctionIdentifier", {
      cell: (info) => <span>{info.getValue()}</span>,
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
  ], [internalt.length]);

  const transactionTable = useReactTable({
    data: formattedTransactions,
    columns: transactionColumns,
    state: {
      globalFilter,
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const internalTransactionTable = useReactTable({
    data: formattedInternalTransactions,
    columns: internalTransactionColumns,
    state: {
      globalFilter,
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const renderTransactionCard = (row: Row<any>) => {
    const data = row.original;

    return (
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-gray-400">Transaction Type</div>
              <div className="text-sm text-white">{txTypeMap[data.TxType]}</div>
            </div>
            <div className="px-2 py-1 rounded bg-[#3d3d3d] bg-opacity-40">
              <span className="text-xs text-[#ffa729]">{inOutMap[data.InOut]}</span>
            </div>
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
              <div className="text-sm text-white">{data.formattedAmount}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400">Fees</div>
              <div className="text-sm text-white">{data.formattedFees}</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{epochToISO(data.TimeStamp)}</div>
          </div>
        </div>
      </div>
    );
  };

  const renderInternalTransactionCard = (row: Row<any>) => {
    const data = row.original;

    return (
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-gray-400">Type</div>
              <div className="text-sm text-white">{atob(String(data.Type))}</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <Link 
              href={"/tx/0x" + decodeBase64ToHexadecimal(data.Hash)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + decodeBase64ToHexadecimal(data.Hash)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            <Link 
              href={"/address/0x" + decodeBase64ToHexadecimal(data.From)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + decodeBase64ToHexadecimal(data.From)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            <Link 
              href={"/address/0x" + decodeBase64ToHexadecimal(data.To)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + decodeBase64ToHexadecimal(data.To)}
            </Link>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-gray-400">Value</div>
              <div className="text-sm text-white">{data.formattedValue}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400">Gas Used</div>
              <div className="text-sm text-white">{data.GasUsed} Units</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{epochToISO(data.BlockTimestamp)}</div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Status</div>
            <div className="text-sm text-white">{data.Output === 1 ? "Success" : "Failure"}</div>
          </div>
        </div>
      </div>
    );
  };

  if (!mounted) {
    return null;
  }

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

      <div className="overflow-x-auto">
        {windowWidth < 768 ? (
          <div className="overflow-hidden">
            {showInternal
              ? internalTransactionTable.getRowModel().rows.map((row) => renderInternalTransactionCard(row))
              : transactionTable.getRowModel().rows.map((row) => renderTransactionCard(row))
            }
          </div>
        ) : (
          <table className="w-full">
            <thead>
              {showInternal
                ? renderTableHeader(internalTransactionTable)
                : renderTableHeader(transactionTable)
              }
            </thead>
            <tbody>
              {showInternal
                ? renderTableBody(internalTransactionTable)
                : renderTableBody(transactionTable)
              }
            </tbody>
          </table>
        )}
      </div>

      <div className="p-4 border-t border-[#3d3d3d]">
        <div className="flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2">
            {showInternal ? (
              <>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => internalTransactionTable.setPageIndex(0)}
                  disabled={!internalTransactionTable.getCanPreviousPage()}
                >
                  {"<<"}
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => internalTransactionTable.previousPage()}
                  disabled={!internalTransactionTable.getCanPreviousPage()}
                >
                  Previous
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => internalTransactionTable.nextPage()}
                  disabled={!internalTransactionTable.getCanNextPage()}
                >
                  Next
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => internalTransactionTable.setPageIndex(internalTransactionTable.getPageCount() - 1)}
                  disabled={!internalTransactionTable.getCanNextPage()}
                >
                  {">>"}
                </button>
              </>
            ) : (
              <>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => transactionTable.setPageIndex(0)}
                  disabled={!transactionTable.getCanPreviousPage()}
                >
                  {"<<"}
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => transactionTable.previousPage()}
                  disabled={!transactionTable.getCanPreviousPage()}
                >
                  Previous
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => transactionTable.nextPage()}
                  disabled={!transactionTable.getCanNextPage()}
                >
                  Next
                </button>
                <button
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
                  onClick={() => transactionTable.setPageIndex(transactionTable.getPageCount() - 1)}
                  disabled={!transactionTable.getCanNextPage()}
                >
                  {">>"}
                </button>
              </>
            )}
          </div>
          <div className="text-sm text-gray-400">
            Page {showInternal
              ? internalTransactionTable.getState().pagination.pageIndex + 1
              : transactionTable.getState().pagination.pageIndex + 1} of{" "}
            {showInternal
              ? internalTransactionTable.getPageCount()
              : transactionTable.getPageCount()}
          </div>
        </div>
      </div>
    </div>
  );
}
