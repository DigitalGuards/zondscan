"use client";

import { useState, useEffect, useMemo } from "react";
import axios from "axios";
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getFilteredRowModel,
  getCoreRowModel,
  getPaginationRowModel,
} from "@tanstack/react-table";
import type {
  Row,
  Table,
  HeaderGroup,
  Header,
  Cell,
  ColumnDef
} from "@tanstack/react-table";
import { formatAmount, formatTimestamp, normalizeHexString, formatAddress, formatTokenAmount } from "../lib/helpers";
import DebouncedInput from "./DebouncedInput";
import { DownloadBtn, DownloadBtnInternal } from "./DownloadBtn";
import Link from "next/link";
import config from "../../config";
import type { Transaction, InternalTransaction } from "@/app/types";

// Token transfer rows for the new "Token Transfers" tab. Mirrors the backend's
// models.TokenTransfer (json tags). All amount math is BigInt-safe via
// formatTokenAmount so an NFT row's "1" renders identically to an ERC-20 row's
// "12345600000000000000".
interface TokenTransferRow {
  contractAddress: string;
  from: string;
  to: string;
  amount: string;
  blockNumber: string;
  txHash: string;
  logIndex?: string;
  timestamp: string;
  tokenSymbol: string;
  tokenDecimals: number;
  tokenName: string;
  transferType: string;
  tokenStandard?: string;
  tokenID?: string;
}

const truncateMiddle = (str: string, startChars = 8, endChars = 8): string => {
  if (str.length <= startChars + endChars) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
};

// Moved outside component to avoid recreating on each render
const IN_OUT_MAP = ["Out", "In"] as const;
const TX_TYPE_MAP = ["Coinbase", "Attest", "Transfer", "Stake"] as const;

// Create column helpers outside component to avoid type inference issues
const columnHelper = createColumnHelper<Transaction & { formattedAmount: string; formattedFees: string }>();
const internalColumnHelper = createColumnHelper<InternalTransaction & { formattedValue: string }>();
const tokenTransferColumnHelper = createColumnHelper<TokenTransferRow & { formattedAmount: string; tsSeconds: number }>();

interface TableProps {
  transactions: Transaction[];
  internalt: InternalTransaction[];
  // Required for the lazy-loaded "Token Transfers" tab. The tab issues
  // /address/:addr/token-transfers, paginated client-side after a single
  // 250-row fetch so the search input filters across all rows the holder
  // has touched without round-tripping per page. When absent the tab is
  // hidden, callers outside the address page don't surface it.
  tokenTransferAddress?: string;
}

type TableInstance<T> = Table<T>;

// Generic over any row shape so the Token Transfers tab can reuse the same
// renderer without expanding the union, the heads/bodies don't read row
// fields, they only walk the table's own header/cell descriptors.
const renderTableHeader = <T,>(
  table: TableInstance<T>
): JSX.Element[] => {
  return table.getHeaderGroups().map((headerGroup: HeaderGroup<T>) => (
    <tr key={headerGroup.id} className="border-b border-[#3d3d3d]">
      {headerGroup.headers.map((header: Header<T, unknown>) => (
        <th
          key={header.id}
          scope="col"
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

const renderTableBody = <T,>(
  table: TableInstance<T>
): JSX.Element[] => {
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
  // Use PaidFees if available (decimal format)
  if (typeof tx.PaidFees === 'number') {
    return tx.PaidFees;
  }
  
  // Fallback to manual calculation only if PaidFees is not available
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

type TabKey = "transactions" | "internal" | "tokenTransfers";

export default function TanStackTable({ transactions, internalt, tokenTransferAddress }: TableProps): JSX.Element | null {
  const [mounted, setMounted] = useState(false);
  const [windowWidth, setWindowWidth] = useState(0);
  const [globalFilter, setGlobalFilter] = useState("");
  // Default to the first tab with rows. Falling back to "transactions"
  // would show an empty table for contract-call-only addresses (no native
  // txs but plenty of internal calls / token transfers). Token transfers
  // are lazy-fetched, so the only signal we have here is "is the page
  // wired to fetch them" (tokenTransferAddress is set), not "are there
  // any". If the fetch later returns zero rows the tab still shows the
  // empty-state message, which is fine.
  const initialActiveTab: TabKey =
    transactions.length > 0
      ? "transactions"
      : internalt.length > 0
        ? "internal"
        : tokenTransferAddress
          ? "tokenTransfers"
          : "transactions";
  const [activeTab, setActiveTab] = useState<TabKey>(initialActiveTab);
  const [tokenTransfers, setTokenTransfers] = useState<TokenTransferRow[]>([]);
  // Server-reported unbounded count for the tab badge. tokenTransfers.length
  // only reflects what was paginated into memory (capped at 250), which would
  // misleadingly stop growing for very active addresses.
  const [tokenTransfersTotal, setTokenTransfersTotal] = useState(0);
  const [tokenTransfersLoaded, setTokenTransfersLoaded] = useState(false);
  const [tokenTransfersLoading, setTokenTransfersLoading] = useState(false);
  const [tokenTransfersError, setTokenTransfersError] = useState<string | null>(null);

  // When the addressee changes mid-mount (Next.js client-side route from
  // /address/A to /address/B reuses this component), wipe the lazy-fetched
  // token-transfer state so the new address triggers a fresh fetch instead
  // of rendering the previous holder's rows. Same goes for the active tab
  // and the search filter — both should reset to the new address's default.
  useEffect(() => {
    setTokenTransfers([]);
    setTokenTransfersTotal(0);
    setTokenTransfersLoaded(false);
    setTokenTransfersLoading(false);
    setTokenTransfersError(null);
    const nextDefault: TabKey =
      transactions.length > 0
        ? "transactions"
        : internalt.length > 0
          ? "internal"
          : tokenTransferAddress
            ? "tokenTransfers"
            : "transactions";
    setActiveTab(nextDefault);
    setGlobalFilter("");
    // Length-based deps keep the effect stable across renders that pass new
    // array identities for the same address.
  }, [tokenTransferAddress, transactions.length, internalt.length]);

  useEffect(() => {
    setMounted(true);
    setWindowWidth(window.innerWidth);
    const handleResize = (): void => setWindowWidth(window.innerWidth);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Lazy-load token transfers on first visit to the tab. One server hit with
  // a generous limit (250) is cheaper than paginating server-side, the address
  // page already pages native txs server-side and a per-tab server round-trip
  // every time the user clicks Next/Prev would feel sluggish vs the local
  // pagination the Transactions tab uses on its 10-row payload. Fetch is
  // gated on tokenTransferAddress so callers without an address can't trip
  // a no-op fetch.
  useEffect(() => {
    if (!tokenTransferAddress) return;
    if (activeTab !== "tokenTransfers") return;
    if (tokenTransfersLoaded || tokenTransfersLoading) return;

    let cancelled = false;
    (async () => {
      setTokenTransfersLoading(true);
      setTokenTransfersError(null);
      try {
        // page-1-indexed on the wire (matches sibling /address routes); we
        // ask for the first page at limit=250 to give the search box something
        // useful to chew on without paging through the server.
        const res = await axios.get(
          `${config.handlerUrl}/address/${tokenTransferAddress}/token-transfers`,
          { params: { page: 1, limit: 250 } }
        );
        if (cancelled) return;
        const rows: TokenTransferRow[] = Array.isArray(res.data?.transfers) ? res.data.transfers : [];
        const total = typeof res.data?.total === "number" ? res.data.total : rows.length;
        setTokenTransfers(rows);
        setTokenTransfersTotal(total);
        setTokenTransfersLoaded(true);
      } catch (err) {
        if (cancelled) return;
        console.error("Failed to load token transfers:", err);
        setTokenTransfersError("Failed to load token transfers");
      } finally {
        if (!cancelled) setTokenTransfersLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeTab, tokenTransferAddress, tokenTransfersLoaded, tokenTransfersLoading]);

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

  const transactionColumns = useMemo(() => [
    columnHelper.accessor(() => "", {
      id: "Number",
      cell: (info) => <span>{transactions.length - info.row.index}</span>,
      header: "Number",
    }),
    columnHelper.accessor("InOut", {
      cell: (info) => <span>{IN_OUT_MAP[info.getValue()]}</span>,
      header: "In/Out",
    }),
    columnHelper.accessor("TxType", {
      cell: (info) => <span>{TX_TYPE_MAP[info.getValue()]}</span>,
      header: "Transaction Type",
    }),
    columnHelper.accessor((row) => ({ from: row.From, to: row.To }), {
      id: "Addresses",
      cell: (info) => {
        const { from, to } = info.getValue();
        const fromAddress = from ? formatAddress("0x" + normalizeHexString(from)) : "";
        const toAddress = to ? formatAddress("0x" + normalizeHexString(to)) : "";
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
        const fullHash = "0x" + normalizeHexString(info.getValue());
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
      cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
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
  ], [transactions.length, columnHelper]);

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
        const fullAddress = formatAddress("0x" + normalizeHexString(info.getValue()));
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
        const fullAddress = formatAddress("0x" + normalizeHexString(info.getValue()));
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
        const fullHash = "0x" + normalizeHexString(info.getValue());
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
      cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      header: "Timestamp",
    }),
    internalColumnHelper.accessor("Output", {
      cell: (info) => <span>{info.getValue() === 1 ? "Success" : "Failure"}</span>,
      header: "Status",
    }),
  ], [internalt.length, internalColumnHelper]);

  // Token transfers, normalised once per fetch. tsSeconds turns the syncer's
  // hex timestamp into the seconds-int formatTimestamp expects; falling back
  // to 0 yields formatTimestamp's empty-string sentinel so a missing field
  // doesn't render "NaN" or "Invalid Date".
  const formattedTokenTransfers = useMemo(() => tokenTransfers.map((row) => {
    const decimals = typeof row.tokenDecimals === "number" ? row.tokenDecimals : 0;
    const formattedAmount = formatTokenAmount(row.amount, decimals);
    let tsSeconds = 0;
    if (row.timestamp) {
      try {
        tsSeconds = row.timestamp.startsWith("0x") ? parseInt(row.timestamp, 16) : parseInt(row.timestamp, 10);
        if (Number.isNaN(tsSeconds)) tsSeconds = 0;
      } catch {
        tsSeconds = 0;
      }
    }
    return { ...row, formattedAmount, tsSeconds };
  }), [tokenTransfers]);

  const tokenTransferColumns = useMemo(() => [
    // Token column: name + standard badge + per-id tag for NFTs. Single
    // cell with two lines so the table stays compact on the address page.
    tokenTransferColumnHelper.accessor((row) => ({
      name: row.tokenName,
      symbol: row.tokenSymbol,
      standard: row.tokenStandard,
      tokenID: row.tokenID,
      contractAddress: row.contractAddress,
    }), {
      id: "Token",
      cell: (info) => {
        const { name, symbol, standard, tokenID, contractAddress } = info.getValue();
        const label = name || symbol || (contractAddress ? truncateMiddle(contractAddress) : "Token");
        // Surface the QRC-X branding on the row badge (DB rows stay ERC-X)
        // so users reading the explorer see consistent QRL-flavoured terminology.
        const badge = standard ? standard.replace(/^ERC-/, "QRC-") : "Token";
        return (
          <div className="flex flex-col">
            <Link
              href={`/address/${contractAddress}`}
              className="text-[#ffa729] hover:text-[#ffb954] font-medium"
              title={contractAddress}
            >
              {label}
            </Link>
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <span className="font-mono">{badge}</span>
              {tokenID && <span className="font-mono">#{tokenID}</span>}
              {symbol && symbol !== name && <span className="font-mono">({symbol})</span>}
            </div>
          </div>
        );
      },
      header: "Token",
    }),
    tokenTransferColumnHelper.accessor((row) => ({ from: row.from, to: row.to }), {
      id: "Parties",
      cell: (info) => {
        const { from, to } = info.getValue();
        return (
          <div className="flex flex-col gap-1">
            {from && (
              <div className="flex items-center gap-1">
                <span className="text-gray-400 text-sm">From:</span>
                <Link href={`/address/${from}`} title={from} className="text-[#ffa729] hover:text-[#ffb954]">
                  {truncateMiddle(from)}
                </Link>
              </div>
            )}
            {to && (
              <div className="flex items-center gap-1">
                <span className="text-gray-400 text-sm">To:</span>
                <Link href={`/address/${to}`} title={to} className="text-[#ffa729] hover:text-[#ffb954]">
                  {truncateMiddle(to)}
                </Link>
              </div>
            )}
          </div>
        );
      },
      header: "From/To",
    }),
    tokenTransferColumnHelper.accessor("formattedAmount", {
      cell: (info) => {
        const row = info.row.original;
        const symbol = row.tokenSymbol;
        return (
          <span className="font-mono">
            {info.getValue()}
            {symbol && <span className="text-gray-500 ml-2">{symbol}</span>}
          </span>
        );
      },
      header: "Amount",
    }),
    tokenTransferColumnHelper.accessor("txHash", {
      cell: (info) => {
        const fullHash = info.getValue();
        return (
          <Link href={`/tx/${fullHash}`} title={fullHash} className="text-[#ffa729] hover:text-[#ffb954]">
            {truncateMiddle(fullHash)}
          </Link>
        );
      },
      header: "Transaction Hash",
    }),
    tokenTransferColumnHelper.accessor("tsSeconds", {
      cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      header: "Timestamp",
    }),
  ], []);

  const transactionTable = useReactTable({
    data: formattedTransactions,
    // @ts-expect-error - ColumnDef types conflict with index signature in Transaction type
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
    // @ts-expect-error - ColumnDef types conflict with index signature in InternalTransaction type
    columns: internalTransactionColumns,
    state: {
      globalFilter,
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const tokenTransferTable = useReactTable({
    data: formattedTokenTransfers,
    columns: tokenTransferColumns as ColumnDef<TokenTransferRow & { formattedAmount: string; tsSeconds: number }>[],
    state: {
      globalFilter,
    },
    onGlobalFilterChange: setGlobalFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  const renderTransactionCard = (row: Row<Transaction & { formattedAmount: string; formattedFees: string }>): JSX.Element => {
    const data = row.original;

    return (
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-gray-400">Transaction Type</div>
              <div className="text-sm text-white">{TX_TYPE_MAP[data.TxType]}</div>
            </div>
            <div className="px-2 py-1 rounded bg-[#3d3d3d] bg-opacity-40">
              <span className="text-xs text-[#ffa729]">{IN_OUT_MAP[data.InOut]}</span>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <Link 
              href={"/tx/0x" + normalizeHexString(data.TxHash)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + normalizeHexString(data.TxHash)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            {data.From && (
              <Link 
                href={"/address/" + formatAddress("0x" + normalizeHexString(data.From))}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {truncateMiddle(formatAddress("0x" + normalizeHexString(data.From)))}
              </Link>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            {data.To && (
              <Link 
                href={"/address/" + formatAddress("0x" + normalizeHexString(data.To))}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {truncateMiddle(formatAddress("0x" + normalizeHexString(data.To)))}
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
            <div className="text-sm text-white">{formatTimestamp(data.TimeStamp)}</div>
          </div>
        </div>
      </div>
    );
  };

  const renderInternalTransactionCard = (row: Row<InternalTransaction & { formattedValue: string }>): JSX.Element => {
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
              href={"/tx/0x" + normalizeHexString(data.Hash)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {"0x" + normalizeHexString(data.Hash)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            <Link 
              href={"/address/" + formatAddress("0x" + normalizeHexString(data.From))}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {truncateMiddle(formatAddress("0x" + normalizeHexString(data.From)))}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            <Link 
              href={"/address/" + formatAddress("0x" + normalizeHexString(data.To))}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {truncateMiddle(formatAddress("0x" + normalizeHexString(data.To)))}
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
            <div className="text-sm text-white">{formatTimestamp(data.BlockTimestamp)}</div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Status</div>
            <div className="text-sm text-white">{data.Output === 1 ? "Success" : "Failure"}</div>
          </div>
        </div>
      </div>
    );
  };

  const renderTokenTransferCard = (row: Row<TokenTransferRow & { formattedAmount: string; tsSeconds: number }>): JSX.Element => {
    const data = row.original;
    const badge = data.tokenStandard ? data.tokenStandard.replace(/^ERC-/, "QRC-") : "Token";
    const label = data.tokenName || data.tokenSymbol || data.contractAddress;
    return (
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-gray-400">Token</div>
              <Link
                href={`/address/${data.contractAddress}`}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {label}
              </Link>
              <div className="text-xs text-gray-400 font-mono">
                {badge}{data.tokenID ? ` #${data.tokenID}` : ""}
              </div>
            </div>
            <div className="px-2 py-1 rounded bg-[#3d3d3d] bg-opacity-40">
              <span className="text-xs text-[#ffa729]">{data.formattedAmount}{data.tokenSymbol ? " " + data.tokenSymbol : ""}</span>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <Link
              href={`/tx/${data.txHash}`}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {data.txHash}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            {data.from && (
              <Link href={`/address/${data.from}`} className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all">
                {truncateMiddle(data.from)}
              </Link>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            {data.to && (
              <Link href={`/address/${data.to}`} className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all">
                {truncateMiddle(data.to)}
              </Link>
            )}
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{formatTimestamp(data.tsSeconds)}</div>
          </div>
        </div>
      </div>
    );
  };

  if (!mounted) {
    return null;
  }

  const showTokenTab = !!tokenTransferAddress;
  const tabId = activeTab === "transactions" ? "tab-transactions" : activeTab === "internal" ? "tab-internal" : "tab-token-transfers";
  const tableLabel = activeTab === "transactions"
    ? "Transaction history"
    : activeTab === "internal"
      ? "Internal transactions"
      : "Token transfers";
  const searchPlaceholder = activeTab === "tokenTransfers"
    ? "Search token transfers..."
    : "Search transactions...";

  // Single switch keeps the table-selection ternaries below readable; without
  // it every render branch (mobile cards, desktop table, paginator, page
  // counter) repeats the same 3-way conditional and drifts over time.
  const activeTable: Table<Transaction & { formattedAmount: string; formattedFees: string }>
    | Table<InternalTransaction & { formattedValue: string }>
    | Table<TokenTransferRow & { formattedAmount: string; tsSeconds: number }> =
      activeTab === "internal"
        ? internalTransactionTable
        : activeTab === "tokenTransfers"
          ? tokenTransferTable
          : transactionTable;

  const renderActiveCards = (): JSX.Element[] => {
    if (activeTab === "internal") {
      return internalTransactionTable.getRowModel().rows.map((row) => renderInternalTransactionCard(row));
    }
    if (activeTab === "tokenTransfers") {
      return tokenTransferTable.getRowModel().rows.map((row) => renderTokenTransferCard(row));
    }
    return transactionTable.getRowModel().rows.map((row) => renderTransactionCard(row));
  };

  const renderActiveHeader = (): JSX.Element[] => {
    if (activeTab === "internal") return renderTableHeader(internalTransactionTable);
    if (activeTab === "tokenTransfers") return renderTableHeader(tokenTransferTable);
    return renderTableHeader(transactionTable);
  };

  const renderActiveBody = (): JSX.Element[] => {
    if (activeTab === "internal") return renderTableBody(internalTransactionTable);
    if (activeTab === "tokenTransfers") return renderTableBody(tokenTransferTable);
    return renderTableBody(transactionTable);
  };

  return (
    <div className="w-full">
      <div className="p-4 border-b border-[#3d3d3d] space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div role="tablist" className="flex items-center flex-wrap gap-2 md:gap-4">
            <button
              id="tab-transactions"
              role="tab"
              aria-selected={activeTab === "transactions"}
              onClick={() => setActiveTab("transactions")}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                activeTab === "transactions"
                  ? "bg-[#ffa729] text-black"
                  : "text-gray-400 hover:text-white"
              }`}
            >
              Transactions
            </button>
            <button
              id="tab-internal"
              role="tab"
              aria-selected={activeTab === "internal"}
              onClick={() => setActiveTab("internal")}
              className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                activeTab === "internal"
                  ? "bg-[#ffa729] text-black"
                  : "text-gray-400 hover:text-white"
              }`}
            >
              Internal Txns
            </button>
            {showTokenTab && (
              <button
                id="tab-token-transfers"
                role="tab"
                aria-selected={activeTab === "tokenTransfers"}
                onClick={() => setActiveTab("tokenTransfers")}
                className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                  activeTab === "tokenTransfers"
                    ? "bg-[#ffa729] text-black"
                    : "text-gray-400 hover:text-white"
                }`}
              >
                Token Transfers
                {tokenTransfersLoaded && (
                  <span className="ml-1 text-xs opacity-75">({tokenTransfersTotal})</span>
                )}
              </button>
            )}
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={globalFilter ?? ""}
              onChange={(value) => setGlobalFilter(String(value))}
              className="px-4 py-2 text-sm bg-[#1a1a1a] border border-[#3d3d3d] rounded-lg focus:outline-none focus:border-[#ffa729] text-white w-full md:w-auto"
              placeholder={searchPlaceholder}
            />
            {activeTab === "internal" ? (
              <DownloadBtnInternal data={internalt} />
            ) : activeTab === "transactions" ? (
              <DownloadBtn data={transactions} />
            ) : null}
          </div>
        </div>
      </div>

      <div role="tabpanel" aria-labelledby={tabId} className="overflow-x-auto">
        {activeTab === "tokenTransfers" && tokenTransfersLoading && (
          <div className="p-8 text-center text-gray-400">Loading token transfers...</div>
        )}
        {activeTab === "tokenTransfers" && !tokenTransfersLoading && tokenTransfersError && (
          <div className="p-8 text-center text-red-400">{tokenTransfersError}</div>
        )}
        {activeTab === "tokenTransfers" && !tokenTransfersLoading && !tokenTransfersError && tokenTransfersLoaded && tokenTransfers.length === 0 && (
          <div className="p-8 text-center text-gray-400">No token transfers for this address.</div>
        )}
        {(activeTab !== "tokenTransfers" || (tokenTransfersLoaded && tokenTransfers.length > 0)) && (
          windowWidth < 768 ? (
            <div className="overflow-hidden">
              {renderActiveCards()}
            </div>
          ) : (
            <table aria-label={tableLabel} className="w-full">
              <thead>{renderActiveHeader()}</thead>
              <tbody>{renderActiveBody()}</tbody>
            </table>
          )
        )}
      </div>

      <div className="p-4 border-t border-[#3d3d3d]">
        <div className="flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-2">
            <button
              aria-label="Go to first page"
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => activeTable.setPageIndex(0)}
              disabled={!activeTable.getCanPreviousPage()}
            >
              {"<<"}
            </button>
            <button
              aria-label="Go to previous page"
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => activeTable.previousPage()}
              disabled={!activeTable.getCanPreviousPage()}
            >
              Previous
            </button>
            <button
              aria-label="Go to next page"
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => activeTable.nextPage()}
              disabled={!activeTable.getCanNextPage()}
            >
              Next
            </button>
            <button
              aria-label="Go to last page"
              className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
              onClick={() => activeTable.setPageIndex(activeTable.getPageCount() - 1)}
              disabled={!activeTable.getCanNextPage()}
            >
              {">>"}
            </button>
          </div>
          <div className="text-sm text-gray-400">
            Page {activeTable.getState().pagination.pageIndex + 1} of {Math.max(1, activeTable.getPageCount())}
          </div>
        </div>
      </div>
    </div>
  );
}
