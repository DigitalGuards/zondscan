'use client';

import { useState, useEffect } from 'react';
import axios from "axios";
import {
  DataGrid,
  GridToolbar,
} from '@mui/x-data-grid';
import type {
  GridColDef,
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

// Helper functions to decode values to strings (for data storage)
const decodeHash = (hash: string): string => {
  return "0x" + decodeToHex(hash);
};

const decodeAddress = (address: string): string => {
  const rawAddress = "0x" + decodeToHex(address);
  return formatAddress(rawAddress);
};

const ConvertTime = (value: any, row: Transaction): string => {
  const timestamp = row.TimeStamp;
  const date = new Date(timestamp * 1000);
  // Use UTC to avoid hydration mismatch
  const day = date.getUTCDate().toString().padStart(2, '0');
  const month = (date.getUTCMonth() + 1).toString().padStart(2, '0');
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours().toString().padStart(2, '0');
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  return `${day}/${month}/${year}, ${hours}:${minutes}:${seconds}`;
};

const Divide = (value: any, row: Transaction): number => {
  return row.Amount / 1000000000;
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
          Address: decodeAddress(item.Address),
          TxHash: decodeHash(item.TxHash),
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
      valueGetter: (_value: any, row: Transaction) => row.Address,
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
      valueGetter: (_value: any, row: Transaction) => row.TxHash,
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
