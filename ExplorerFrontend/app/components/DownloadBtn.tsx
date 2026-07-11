'use client';

import { useState } from 'react';
import { formatTimestamp, normalizeHexString, formatAddress } from '../lib/helpers';
import type { MouseEvent } from 'react';
import type { DownloadBtnProps, DownloadBtnInternalProps } from '@/app/types';

function escapeCSV(value: string | number): string {
  const str = String(value);
  if (str.includes(',') || str.includes('"') || str.includes('\n')) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  return str;
}

function downloadCSV(data: Record<string, string | number>[], fileName: string): void {
  if (data.length === 0) return;

  const headers = Object.keys(data[0]);
  const csvRows = [
    headers.map(escapeCSV).join(','),
    ...data.map(row => headers.map(h => escapeCSV(row[h] ?? '')).join(','))
  ];

  const blob = new Blob([csvRows.join('\n')], { type: 'text/csv;charset=utf-8;' });
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = fileName;
  link.click();
  URL.revokeObjectURL(link.href);
}

export function DownloadBtn({ data = [], fileName, getData }: DownloadBtnProps): JSX.Element {
  // Guards double-clicks while a getData provider is fetching the full
  // history; also swaps the label so the wait is visible.
  const [busy, setBusy] = useState(false);

  const handleDownload = async (e: MouseEvent<HTMLButtonElement>): Promise<void> => {
    e.preventDefault();
    if (busy) return;

    let datas = data?.length ? data : [];
    if (getData) {
      setBusy(true);
      try {
        datas = await getData();
      } catch (error) {
        console.error('Failed to fetch rows for download:', error);
        window.alert('Failed to download the full history. Please try again.');
        return;
      } finally {
        setBusy(false);
      }
    }

    const convertedData = datas.map(item => {
      const convertedItem: Record<string, string | number> = {};
      for (const key in item) {
        if (key === 'ID') {
          continue;
        }

        const value = item[key];
        if (value === undefined) continue;

        if (key === 'Amount' || key === 'Paidfees') {
          convertedItem[key] = Number(value);
        } else if (key === 'TimeStamp') {
          convertedItem[key] = formatTimestamp(Number(value));
        } else if (key === 'Address' || key === 'TxHash') {
          if (typeof value === 'string') {
            convertedItem[key] = key === 'Address' ?
              formatAddress("0x" + normalizeHexString(value)) :
              "0x" + normalizeHexString(value);
          }
        } else if (key === 'From' || key === 'To') {
          if (typeof value === 'string') {
            convertedItem[key] = value ?
              formatAddress("0x" + normalizeHexString(value)) :
              "No Address Found";
          }
        } else {
          convertedItem[key] = value;
        }
      }
      return convertedItem;
    });

    downloadCSV(convertedData, fileName ? `${fileName}.csv` : "data.csv");
  };

  return (
    <button
      type="button"
      className="px-4 py-2 text-sm font-medium rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] hover:text-white transition-colors disabled:opacity-60"
      onClick={(e) => { void handleDownload(e); }}
      disabled={busy}
    >
      {busy ? 'Preparing...' : 'Download'}
    </button>
  );
}

export function DownloadBtnInternal({ data = [], fileName, getData }: DownloadBtnInternalProps): JSX.Element {
  const [busy, setBusy] = useState(false);

  const handleDownload = async (e: MouseEvent<HTMLButtonElement>): Promise<void> => {
    e.preventDefault();
    if (busy) return;

    let datas = data?.length ? data : [];
    if (getData) {
      setBusy(true);
      try {
        datas = await getData();
      } catch (error) {
        console.error('Failed to fetch rows for download:', error);
        window.alert('Failed to download the full history. Please try again.');
        return;
      } finally {
        setBusy(false);
      }
    }

    const skipKeys = new Set([
      'ID', 'CallType', 'Calls', 'TraceAdd', 'Address',
      'TraceAddress', 'InOut'
    ]);

    const convertedData = datas.map(item => {
      const convertedItem: Record<string, string | number> = {};
      for (const key in item) {
        if (skipKeys.has(key)) {
          continue;
        }

        const value = item[key];
        if (value === undefined) continue;

        try {
          if (['Value', 'Gas', 'GasUsed', 'AmountFunctionIdentifier'].includes(key)) {
            convertedItem[key] = Number(value);
          } else if (key === 'Type' && typeof value === 'string') {
            convertedItem[key] = atob(value);
          } else if (key === 'BlockTimestamp') {
            convertedItem[key] = formatTimestamp(Number(value));
          } else if (['AddressFunctionIdentifier', 'From', 'To', 'Hash'].includes(key)) {
            if (typeof value === 'string') {
              convertedItem[key] = "0x" + normalizeHexString(value);
            }
          } else if (typeof value !== 'object') { // Skip arrays and objects
            convertedItem[key] = value;
          }
        } catch (error) {
          console.error(`Error processing key ${key}:`, error);
          convertedItem[key] = 'Error processing data';
        }
      }
      return convertedItem;
    });

    downloadCSV(convertedData, fileName ? `${fileName}.csv` : "data.csv");
  };

  return (
    <button
      type="button"
      className="px-4 py-2 text-sm font-medium rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] hover:text-white transition-colors disabled:opacity-60"
      onClick={(e) => { void handleDownload(e); }}
      disabled={busy}
    >
      {busy ? 'Preparing...' : 'Download'}
    </button>
  );
}

export default DownloadBtn;
