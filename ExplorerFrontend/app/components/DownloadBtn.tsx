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

export function DownloadBtn({ data = [], fileName }: DownloadBtnProps): JSX.Element {
  const handleDownload = (e: MouseEvent<HTMLButtonElement>): void => {
    e.preventDefault();
    const datas = data?.length ? data : [];

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
      className="download-btn"
      onClick={handleDownload}
    >
      Download
    </button>
  );
}

export function DownloadBtnInternal({ data = [], fileName }: DownloadBtnInternalProps): JSX.Element {
  const handleDownload = (e: MouseEvent<HTMLButtonElement>): void => {
    e.preventDefault();
    const datas = data?.length ? data : [];

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
      className="download-btn"
      onClick={handleDownload}
    >
      Download
    </button>
  );
}

export default DownloadBtn;
