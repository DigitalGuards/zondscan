import * as XLSX from 'xlsx';
import { decodeBase64ToHexadecimal, formatTimestamp } from '../lib/helpers';
import { MouseEvent } from 'react';
import type { DownloadBtnProps, DownloadBtnInternalProps } from './types';

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
            convertedItem[key] = "0x" + decodeBase64ToHexadecimal(value);
          }
        } else if (key === 'From' || key === 'To') {
          if (typeof value === 'string') {
            convertedItem[key] = value ? "0x" + decodeBase64ToHexadecimal(value) : "No Address Found";
          }
        } else {
          convertedItem[key] = value;
        }
      }
      return convertedItem;
    });
            
    const worksheet = XLSX.utils.json_to_sheet(convertedData);
    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, worksheet, "Sheet1");
    XLSX.writeFile(workbook, fileName ? `${fileName}.xlsx` : "data.xlsx");
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
              convertedItem[key] = "0x" + decodeBase64ToHexadecimal(value);
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
            
    const worksheet = XLSX.utils.json_to_sheet(convertedData);
    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, worksheet, "Sheet1");
    XLSX.writeFile(workbook, fileName ? `${fileName}.xlsx` : "data.xlsx");
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
