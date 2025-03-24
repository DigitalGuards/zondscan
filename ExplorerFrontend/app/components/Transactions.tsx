'use client';

import { useState, useEffect } from 'react';
import axios from "axios";
import { 
  DataGrid, 
  GridToolbar, 
  GridColDef, 
  GridValueGetterParams,
  GridRenderCellParams 
} from '@mui/x-data-grid';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import Link from "next/link";
import config from '../../config';
import { decodeToHex, formatAddress } from '../lib/helpers';

interface Transaction {
  id: number;
  TxHash: string;
  Address: string;
  Amount: number;
  InOut: number;
  TimeStamp: number;
  TxType: string;
}

interface DecoderParams {
  row: {
    TxHash: string;
    Address: string;
  };
}

const Decoder = (params: DecoderParams): JSX.Element => {
  const output = "0x" + decodeToHex(params.row.TxHash);
  const url = `${config.siteUrl}/tx/${output}`;
  return <Link href={url}>{output}</Link>;
};

const DecoderAddress = (params: DecoderParams): JSX.Element => {
  const rawAddress = "0x" + decodeToHex(params.row.Address);
  const formattedAddress = formatAddress(rawAddress);
  return <Link href={`${config.siteUrl}/address/${formattedAddress}`}>{formattedAddress}</Link>;
};

const ConvertTime = (params: GridValueGetterParams<Transaction>): string => {
  const timestamp = params.row.TimeStamp;
  const date = new Date(timestamp * 1000);
  
  return date.toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
};

const Divide = (params: GridValueGetterParams<Transaction>): number => {
  return params.row.Amount / 1000000000;
};

export default function Transactions(): JSX.Element {
  const [transactions, setTransactions] = useState<Transaction[] | null>(null);

  useEffect(() => {
    const fetchTransactions = async (): Promise<void> => {
      try {
        const response = await axios.get(`${config.handlerUrl}/transactions`);
        const data = response.data.response;
    
        const transformedData = data.map((item: any, index: number) => ({
          id: index,
          ...item,
          Address: DecoderAddress({row: item}),
          TxHash: Decoder({row: item}),
        }));
    
        setTransactions(transformedData);
      } catch (error) {
        console.error('Error fetching transactions:', error);
      }
    };

    fetchTransactions();
  }, []);

  const columns: GridColDef[] = [
    {
      field: 'Address',
      headerName: 'Address',
      flex: 1,
      valueGetter: (params: GridValueGetterParams<Transaction>) => params.row.Address,
      renderCell: (params: GridRenderCellParams<Transaction>) => (
        <Link href={`${config.siteUrl}/address/${params.row.Address}`}>
          {params.row.Address}
        </Link>
      ),
    },
    {
      field: 'Amount',
      headerName: 'Amount',
      flex: 1,
      valueGetter: Divide
    },
    {
      field: 'InOut',
      headerName: 'Transfer In/out',
      flex: 1
    },
    {
      field: 'TimeStamp',
      headerName: 'Timestamp',
      flex: 1,
      valueGetter: ConvertTime
    },
    {
      field: 'TxHash',
      headerName: 'Transaction hash',
      flex: 1,
      valueGetter: (params: GridValueGetterParams<Transaction>) => params.row.TxHash,
      renderCell: (params: GridRenderCellParams<Transaction>) => (
        <Link href={`${config.siteUrl}/tx/${params.row.TxHash}`}>
          {params.row.TxHash}
        </Link>
      ),
    },
    {
      field: 'TxType',
      headerName: 'Tx Type',
      flex: 1
    },
  ];

  return (
    <Box m={2}>
      <Typography variant="h6" component="div" align="center">
        Transactions
      </Typography>
      <Box height={500} width="100%">
        {transactions && (
          <DataGrid
            rows={transactions}
            columns={columns}
            initialState={{
              pagination: {
                paginationModel: {
                  pageSize: 10,
                },
              },
            }}
            pageSizeOptions={[5, 10, 25]}
            rowHeight={50}
            slots={{
              toolbar: GridToolbar,
            }}
          />
        )}
      </Box>
    </Box>
  );
}
