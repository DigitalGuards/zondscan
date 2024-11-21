import * as XLSX from 'xlsx'
import {decodeBase64ToHexadecimal, formatTimestamp} from '../lib/helpers.js'

export const DownloadBtn = ({ data = [], fileName }) => {
  return (
    <button
      className="download-btn"
      onClick={() => {
        const datas = data?.length ? data : [];

        let convertedData = datas.map(item => {
          let convertedItem = {};
          for (const key in item) {
            if (key === 'ID') {
              continue;
            } else if (key === 'Amount') {
              convertedItem[key] = item[key];
            } else if (key === 'TimeStamp') {
              convertedItem[key] = formatTimestamp(item[key]);
            } else if (key === 'Address' || key === 'From' || key === 'TxHash') {
              convertedItem[key] = "0x" + decodeBase64ToHexadecimal(item[key]);
            } else if (key === 'To') {
              if (item[key] != null){
                convertedItem[key] = "0x" + decodeBase64ToHexadecimal(item[key]);
              } else{
                convertedItem[key] = "No To key Found"
              }
            }else {
              convertedItem[key] = item[key];
            }
          }
          return convertedItem;
        });
                
        const worksheet = XLSX.utils.json_to_sheet(convertedData);
        const workbook = XLSX.utils.book_new();
        XLSX.utils.book_append_sheet(workbook, worksheet, "Sheet1");
        XLSX.writeFile(workbook, fileName ? `${fileName}.xlsx` : "data.xlsx");
      }}
    >
      Download
    </button>
  );
};

export const DownloadBtnInternal = ({ data = [], fileName }) => {
  return (
    <button
      className="download-btn"
      onClick={() => {
        const datas = data?.length ? data : [];

        let convertedItem = {};
        
        let convertedData = datas.map(item => {
          for (const key in item) {
            if (key === 'ID' || key === "CallType" || key === "Calls" || key === "TraceAdd" || key === "Address") {
              continue;
            } else if (key === "CallType" || key === "Calls" || key === "TraceAdd" || key === "Address") {
              continue;
            } else if (key === "Calls" || key === "TraceAdd" || key === "Address") {
              continue;
            } else if (key === "TraceAddress") {
              continue;
            } else if (key === "Address") {
              continue;
            } else if (key === "InOut") {
              continue;
            } else if (key === 'Amount') {
              convertedItem[key] = item[key];
            } else if (key === 'Type') {
              convertedItem[key] = atob(item[key]);
            } else if (key === 'BlockTimestamp') {
              convertedItem[key] = formatTimestamp(item[key]);
            } else if (key === 'AddressFunctionIdentifier') {
              convertedItem[key] = "0x" + decodeBase64ToHexadecimal(item[key]);
            } else if (key === 'From') {
              convertedItem[key] = "0x" + decodeBase64ToHexadecimal(item[key]);
            } else if (key === 'To') {
              convertedItem[key] = "0x" + decodeBase64ToHexadecimal(item[key]);
            }else {
              convertedItem[key] = item[key];
            }
          }
          console.log(convertedItem);
          return convertedItem;
        });
                
        const worksheet = XLSX.utils.json_to_sheet(convertedData);
        const workbook = XLSX.utils.book_new();
        XLSX.utils.book_append_sheet(workbook, worksheet, "Sheet1");
        XLSX.writeFile(workbook, fileName ? `${fileName}.xlsx` : "data.xlsx");
      }}
    >
      Download
    </button>
  );
};