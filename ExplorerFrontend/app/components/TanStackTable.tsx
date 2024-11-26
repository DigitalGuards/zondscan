"use client";

import { useState } from "react";
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getFilteredRowModel,
  getCoreRowModel,
  getPaginationRowModel,
  ColumnDef
} from "@tanstack/react-table";
import { decodeBase64ToHexadecimal, epochToISO, toFixed } from "../lib/helpers";
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
    columnHelper.accessor("Address", {
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
    columnHelper.accessor((row) => toFixed(row.Amount), {
      id: "Amount",
      header: "Amount (QRL)",
    }),
    columnHelper.accessor((row) => toFixed(row.Paidfees), {
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

  const [selectedItem, setSelectedItem] = useState(0);
  const [tableData, setTableData] = useState<TableData>({
    transactions: transactions || [],
    internalTransactions: internalt || []
  });
  const [globalFilter, setGlobalFilter] = useState("");

  const table = useReactTable({
    data: tableData.transactions,
    columns,
    state: {
      globalFilter,
    },
    getFilteredRowModel: getFilteredRowModel(),
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const tableInternalTransactions = useReactTable({
    data: tableData.internalTransactions,
    columns: internalColumns,
    state: {
      globalFilter,
    },
    getFilteredRowModel: getFilteredRowModel(),
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const handleTabChange = (idx: number) => {
    setSelectedItem(idx);
  }

  if (!transactions || transactions.length === 0) {
    return <div>No transactions yet.</div>;
  }

  return (
     <>
      <div className="text-sm mt-5 overflow-x-auto">
        <ul role="tablist" className="w-full border-b flex items-center gap-x-3 overflow-x-auto mb-4">
          <li className={`py-2 border-b-2 ${selectedItem === 0 ? "border-indigo-600 text-indigo-600" : "border-white text-gray-500"}`}>
            <button
              role="tab"
              aria-selected={selectedItem === 0}
              onClick={() => handleTabChange(0)}
              className="py-2.5 px-4 rounded-lg duration-150 hover:text-indigo-600 hover:bg-gray-50 active:bg-gray-100 font-medium"
            >
              Transactions
            </button>
          </li>
          <li className={`py-2 border-b-2 ${selectedItem === 1 ? "border-indigo-600 text-indigo-600" : "border-white text-gray-500"}`}>
            <button
              role="tab"
              aria-selected={selectedItem === 1}
              onClick={() => handleTabChange(1)}
              className="py-2.5 px-4 rounded-lg duration-150 hover:text-indigo-600 hover:bg-gray-50 active:bg-gray-100 font-medium"
            >
              Internal Transactions
            </button>
          </li>
        </ul>
  
        {/* Conditional rendering for Transactions tab */}
        {selectedItem === 0 && (
          tableData.transactions.length > 0 ? (
              <div className="p-2 max-w-full mx-auto overflow-x-auto bg-white rounded-lg shadow-md">
           <div className="flex justify-between mb-4">
             <div className="w-full flex items-center gap-1">
           <DebouncedInput
             value={globalFilter ?? ""}
             onChange={(value: any) => setGlobalFilter(String(value))}
             className="p-2 bg-transparent outline-none border-b-2 w-1/5 focus:w-1/3 duration-300 border-ffa729 hover:border-ffa940 w-full"
             placeholder="Search all columns..."
           />
             </div>
             <div className="flex inline">
             <p>Download all your transactions - </p> 
             <DownloadIcon />
               {/* @ts-ignore */}
               <DownloadBtn data={tableData.transactions} fileName="data" />
             </div>
           </div>
           <table className="w-full table-auto text-left divide-y divide-gray-200">
             <thead className="bg-indigo-100">
               {table.getHeaderGroups().map((headerGroup) => (
                 <tr key={headerGroup.id}>
                   {headerGroup.headers.map((header) => (
                     <th key={header.id} className="capitalize px-3.5 py-2 text-indigo-600">
                       {flexRender(header.column.columnDef.header, header.getContext())}
                     </th>
                   ))}
                 </tr>
               ))}
             </thead>
             <tbody className="text-gray-600">
               {table.getRowModel().rows.length ? (
                 table.getRowModel().rows.map((row, i) => (
                   <tr key={row.id} className={`${i % 2 === 0 ? "bg-gray-100" : "bg-white"}`}>
                     {row.getVisibleCells().map((cell) => (
                       <td key={cell.id} className="px-3.5 py-2 text-gray-900">
                         {flexRender(cell.column.columnDef.cell, cell.getContext())}
                       </td>
                     ))}
                   </tr>
                 ))
               ) : (
                 <tr className="text-center h-32">
                   <td colSpan={12}>No Record Found!</td>
                 </tr>
               )}
             </tbody>
           </table>
       <div className="flex items-center justify-end mt-2 gap-2">
         <button
           onClick={() => {
             table.previousPage();
           }}
           disabled={!table.getCanPreviousPage()}
           className="p-1 border border-gray-300 px-2 disabled:opacity-30"
         >
           {"<"}
         </button>
         <button
           onClick={() => {
             table.nextPage();
           }}
           disabled={!table.getCanNextPage()}
           className="p-1 border border-gray-300 px-2 disabled:opacity-30"
         >
           {">"}
         </button>
  
         <span className="flex items-center gap-1">
           <div>Page</div>
           <strong>
             {table.getState().pagination.pageIndex + 1} of{" "}
             {table.getPageCount()}
           </strong>
         </span>
         <span className="flex items-center gap-1">
           | Go to page:
           <input
             type="number"
             defaultValue={table.getState().pagination.pageIndex + 1}
             onChange={(e) => {
               const page = e.target.value ? Number(e.target.value) - 1 : 0;
               table.setPageIndex(page); 
             }}
             className="border p-1 rounded w-16 bg-transparent"
           />
         </span>
         <select
           value={table.getState().pagination.pageSize}
           onChange={(e) => {
             table.setPageSize(Number(e.target.value));
           }}
           className="p-2 bg-transparent"
         >
           {[10, 20, 30, 50].map((pageSize) => (
             <option key={pageSize} value={pageSize}>
               Show {pageSize}
             </option>
           ))}
         </select>
  
           </div>
         </div>
          ) : (
            <div>No transactions yet.</div>
          )
        )}
  
        {/* Conditional rendering for Internal Transactions tab */}
        {selectedItem === 1 && (
          tableData.internalTransactions.length > 0 ? (
            <div className="p-2 max-w-full mx-auto overflow-x-auto bg-white rounded-lg shadow-md">
            <div className="flex justify-between mb-4">
              <div className="w-full flex items-center gap-1">
            <DebouncedInput
              value={globalFilter ?? ""}
              onChange={(value: any) => setGlobalFilter(String(value))}
              className="p-2 bg-transparent outline-none border-b-2 w-1/5 focus:w-1/3 duration-300 border-ffa729 hover:border-ffa940 w-full"
              placeholder="Search all columns..."
            />
              </div>
              <div className="flex inline">
              <p>Download all your transactions - </p> 
              <DownloadIcon />
                {/* @ts-ignore */}
                <DownloadBtnInternal data={tableData.internalTransactions} fileName="internalTransactions" />
              </div>
            </div>
            <table className="w-full table-auto text-left divide-y divide-gray-200">
              <thead className="bg-indigo-100">
                {tableInternalTransactions.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <th key={header.id} className="capitalize px-3.5 py-2 text-indigo-600">
                        {flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="text-gray-600">
                {tableInternalTransactions.getRowModel().rows.length ? (
                  tableInternalTransactions.getRowModel().rows.map((row, i) => (
                    <tr key={row.id} className={`${i % 2 === 0 ? "bg-gray-100" : "bg-white"}`}>
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id} className="px-3.5 py-2 text-gray-900">
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  ))
                ) : (
                  <tr className="text-center h-32">
                    <td colSpan={12}>No Record Found!</td>
                  </tr>
                )}
              </tbody>
            </table>
        <div className="flex items-center justify-end mt-2 gap-2">
          <button
            onClick={() => {
              tableInternalTransactions.previousPage();
            }}
            disabled={!tableInternalTransactions.getCanPreviousPage()}
            className="p-1 border border-gray-300 px-2 disabled:opacity-30"
          >
            {"<"}
          </button>
          <button
            onClick={() => {
              tableInternalTransactions.nextPage();
            }}
            disabled={!tableInternalTransactions.getCanNextPage()}
            className="p-1 border border-gray-300 px-2 disabled:opacity-30"
          >
            {">"}
          </button>
  
          <span className="flex items-center gap-1">
            <div>Page</div>
            <strong>
              {tableInternalTransactions.getState().pagination.pageIndex + 1} of{" "}
              {tableInternalTransactions.getPageCount()}
            </strong>
          </span>
          <span className="flex items-center gap-1">
            | Go to page:
            <input
              type="number"
              defaultValue={tableInternalTransactions.getState().pagination.pageIndex + 1}
              onChange={(e) => {
                const page = e.target.value ? Number(e.target.value) - 1 : 0;
                tableInternalTransactions.setPageIndex(page); 
              }}
              className="border p-1 rounded w-16 bg-transparent"
            />
          </span>
          <select
            value={tableInternalTransactions.getState().pagination.pageSize}
            onChange={(e) => {
              tableInternalTransactions.setPageSize(Number(e.target.value));
            }}
            className="p-2 bg-transparent"
          >
            {[10, 20, 30, 50].map((pageSize) => (
              <option key={pageSize} value={pageSize}>
                Show {pageSize}
              </option>
            ))}
          </select>
  
            </div>
          </div>
          ) : (
            <div>No Internal transactions yet.</div>
          )
        )}
      </div>
    </>
  );
}
