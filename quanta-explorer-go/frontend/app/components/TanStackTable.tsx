"use client";

import { useState } from "react";
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getFilteredRowModel,
  getCoreRowModel,
  getPaginationRowModel
} from "@tanstack/react-table";
import { decodeBase64ToHexadecimal, epochToISO, toFixed } from "../lib/helpers";
import DebouncedInput from "./DebouncedInput";
import { DownloadIcon } from "./Icons";
import {DownloadBtn, DownloadBtnInternal} from "./DownloadBtn";
import Link from "next/link";

interface Transaction {
  ID: string;
  InOut: number;
  TxType: number;
  Address: string;
  TransactionAddress: string;
  TxHash: string;
  TimeStamp: number;
  Amount: number;
  Paidfees: number;
}

interface InternalTransaction {
  Type: number;
  CallType: string;
  Hash: string;
  From: string;
  To: string;
  Input: string;
  Output: number;
  TraceAddress: Array<number | string>;
  Value: number;
  Gas: number;
  GasUsed: number;
  AddressFunctionIdentifier: string;
  AmountFunctionIdentifier: number;
  BlockTimestamp: number;
}


export default function TanStackTable({ transactions, internalt }) {

  const columnHelper = createColumnHelper();
  const inOutMap = ["Out", "In"];
  const txTypeMap = ["Coinbase", "Attest", "Transfer", "Stake"];

  const columns = [
    columnHelper.accessor((data: Transaction) => "", {
      id: "Number",
      cell: (info) => <span>{transactions.length - info.row.index}</span>,
      header: "Number",
    }),
    columnHelper.accessor((data: Transaction) => data.InOut, {
      cell: (info) => <span>{inOutMap[info.getValue() as number]}</span>,
      header: "In/Out",
    }),
    columnHelper.accessor((data: Transaction) => data.TxType, {
      cell: (info) => <span>{txTypeMap[info.getValue() as number]}</span>,
      header: "Transaction Type",
    }),
    columnHelper.accessor((data: Transaction) => data.Address, {
      cell: (info) => <span><Link href={"/address/" + "0x" + decodeBase64ToHexadecimal(info.getValue()) as any}>{"0x" + decodeBase64ToHexadecimal(info.getValue()) as any}</Link></span>,
      header: "From/To",
    }),
    columnHelper.accessor((data: Transaction) => data.TxHash, {
      cell: (info) => <span><Link href={"/tx/" + "0x" + decodeBase64ToHexadecimal(info.getValue()) as any}>{"0x" + decodeBase64ToHexadecimal(info.getValue()) as any}</Link></span>,
      header: "Transaction Hash",
    }),
    columnHelper.accessor((data: Transaction) => data.TimeStamp, {
      cell: (info) => <span>{epochToISO(info.getValue()) as any}</span>,
      header: "Timestamp",
    }),
    columnHelper.accessor((data: Transaction) => toFixed(data.Amount), {
      cell: (info) => <span>{(info.getValue()) as any}</span>,
      header: "Amount (QRL)",
    }),
    columnHelper.accessor((data: Transaction) => toFixed(data.PaidFees), {
      cell: (info) => <span>{(info.getValue()) as any}</span>,
      header: "Paid Fees (QRL)",
    }),
  ];

  const internalColumns = [
    columnHelper.accessor((data: Transaction) => "", {
      id: "Number",
      cell: (info) => <span>{transactions.length - info.row.index}</span>,
      header: "Number",
    }),
    columnHelper.accessor((data: Transaction) => data.InOut, {
      cell: (info) => <span>{inOutMap[info.getValue() as number]}</span>,
      header: "In/Out",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.Type, {
      cell: (info) => <span>{atob(info.getValue())}</span>,
      header: "Type",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.From, {
      cell: (info) => <span><Link href={"/address/" + "0x" + decodeBase64ToHexadecimal(info.getValue()) as any}>{"0x" + decodeBase64ToHexadecimal(info.getValue()) as any}</Link></span>,
      header: "From",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.To, {
      cell: (info) => <span><Link href={"/address/" + "0x" + decodeBase64ToHexadecimal(info.getValue()) as any}>{"0x" + decodeBase64ToHexadecimal(info.getValue()) as any}</Link></span>,
      header: "To",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.Hash, {
      cell: (info) => <span><Link href={"/tx/" + "0x" + decodeBase64ToHexadecimal(info.getValue()) as any}>{"0x" + decodeBase64ToHexadecimal(info.getValue()) as any}</Link></span>,
      header: "Transaction Hash",
    }),
    columnHelper.accessor((data: InternalTransaction) => toFixed(data.Value), {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Value (QRL)",
    }),
    columnHelper.accessor((data: InternalTransaction) => toFixed(data.GasUsed), {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Gas Used (in Units)",
    }),
    columnHelper.accessor((data: InternalTransaction) => toFixed(data.AmountFunctionIdentifier), {
      cell: (info) => <span>{info.getValue()}</span>,
      header: "Token Units",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.BlockTimestamp, {
      cell: (info) => <span>{epochToISO(info.getValue() as number)}</span>,
      header: "Timestamp",
    }),
    columnHelper.accessor((data: InternalTransaction) => data.Output, {
      cell: (info) => <span>{info.getValue() as number === 1 ? "Success" : "Failure"}</span>,
      header: "Status",
    }),
  ];

  const tableItems = [
    {
      label: "Transactions",
      data: transactions,
    },
    {
      label: "Internal Transactions",
      data: internalt,
    },
    // {
    //   label: "Token Transfers",
    // },
    // {
    //   label: "Produced Blocks",
    // },
    // {
    //   label: "Analytics",
    // },
    // {
    //   label: "Comments",
    // },
  ];

  const [selectedItem, setSelectedItem] = useState(0);
  const [data, setData] = useState(() => transactions ? [...transactions] : []);
  const [internalTransactions, setInternalData] = useState(() => internalt ? [...internalt] : []);
  const [globalFilter, setGlobalFilter] = useState("");

  const table = useReactTable({
    data,
    columns: columns,
    state: {
      globalFilter,
    },
    getFilteredRowModel: getFilteredRowModel(),
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const tableInternalTransactions = useReactTable({
    data,
    columns: internalColumns,
    state: {
      globalFilter,
    },
    getFilteredRowModel: getFilteredRowModel(),
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const handleTabChange = (idx) => {
    setSelectedItem(idx);
    setData(tableItems[idx].data);
    setInternalData(tableItems[idx].data);
  }

  if (!transactions || transactions.length === 0) {
    return <div>No transactions yet.</div>;
  }

  return (
     <>
      <div className="text-sm mt-5 overflow-x-auto">
        <ul role="tablist" className="w-full border-b flex items-center gap-x-3 overflow-x-auto mb-4">
          {tableItems.map((item, idx) => (
            <li key={idx} className={`py-2 border-b-2 ${selectedItem === idx ? "border-indigo-600 text-indigo-600" : "border-white text-gray-500"}`}>
              <button
                role="tab"
                aria-selected={selectedItem === idx}
                onClick={() => handleTabChange(idx)}
                className="py-2.5 px-4 rounded-lg duration-150 hover:text-indigo-600 hover:bg-gray-50 active:bg-gray-100 font-medium"
              >
                {item.label}
              </button>
            </li>
          ))}
        </ul>
  
        {/* Conditional rendering for Transactions tab */}
        {selectedItem === 0 && (
          transactions && transactions.length > 0 ? (
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
               <DownloadBtn data={data} fileName={"data"} />
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
          internalt && internalt.length > 0 ? (
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
                <DownloadBtnInternal data={internalTransactions} fileName={"internalTransactions"} />
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