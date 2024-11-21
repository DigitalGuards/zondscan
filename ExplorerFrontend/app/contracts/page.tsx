"use client"

import * as React from 'react';
import { Buffer } from 'buffer';
import axios from "axios";
import { DataGrid, GridToolbar, GridRenderCellParams } from '@mui/x-data-grid';
import Typography from '@mui/material/Typography';
import { createTheme, ThemeProvider } from '@mui/material/styles';
import Box from '@mui/material/Box';
import config from '../../config.js';

interface ContractData {
  id: number;
  from: string;
  txHash: string;
  pk: string;
  signature: string;
  nonce: string;
  value: string;
  contractAddress: string;
}

const darkTheme = createTheme({
  palette: {
    mode: 'dark',
    background: {
      default: '#1a1b1e',
      paper: '#2c2d31',
    },
    text: {
      primary: '#ffffff',
      secondary: '#9ca3af',
    },
  },
});

const DecoderAddress = (params: { row: { from: string } }): string => {
  const buffer = Buffer.from(params.row.from, 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

const DecoderTxHash = (params: { row: { txHash: string } }): string => {
  const buffer = Buffer.from(params.row.txHash, 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

const Decoder = (params: { row: { pk?: string; signature?: string } }): string => {
  const buffer = Buffer.from(params.row.pk || params.row.signature || '', 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

export default function Contracts() {
  const [post, setPost] = React.useState<ContractData[] | null>(null);
  const [paginationModel, setPaginationModel] = React.useState({
    pageSize: 10,
    page: 0,
  });

  React.useEffect(() => {
    axios.get(config.handlerUrl + "/contracts").then((response) => {
      const data = response.data.response;
  
      const transformedData = data.map((item: any, index: number) => ({
        id: index,
        ...item,
        from: DecoderAddress({row: item}),
        txHash: DecoderTxHash({row: item}),
        pk: Decoder({row: { pk: item.pk }}),
        signature: Decoder({row: { signature: item.signature }}),
        contractAddress: DecoderAddress({row: item})
      }));
  
      setPost(transformedData);
    });
  }, []);

  const columns = [
    {
      field: 'from', 
      headerName: 'From (Contract Creator)', 
      flex: 1,
      minWidth: 200,
      renderCell: (params: GridRenderCellParams<ContractData>) => (
        <a 
          href={`/address/${params.row.from}`}
          className="text-[#ffa729] hover:text-[#ffb954] transition-colors"
        >
          {params.row.from}
        </a>
      ),
    },
    { 
      field: 'txHash', 
      headerName: 'Transaction Hash', 
      flex: 1,
      minWidth: 200,
      renderCell: (params: GridRenderCellParams<ContractData>) => (
        <a 
          href={`/tx/${params.row.txHash}`}
          className="text-[#ffa729] hover:text-[#ffb954] transition-colors"
        >
          {params.row.txHash}
        </a>
      ),
    },
    { 
      field: 'pk', 
      headerName: 'Public Key', 
      flex: 1,
      minWidth: 150,
    },
    { 
      field: 'signature', 
      headerName: 'Signature', 
      flex: 1,
      minWidth: 150,
    },
    { 
      field: 'nonce', 
      headerName: 'Nonce', 
      flex: 0.5,
      minWidth: 100,
    },
    { 
      field: 'value', 
      headerName: 'Value', 
      flex: 0.5,
      minWidth: 100,
    },
    { 
      field: 'contractAddress', 
      headerName: 'Contract Address', 
      flex: 1,
      minWidth: 200,
      renderCell: (params: GridRenderCellParams<ContractData>) => (
        <a 
          href={`/address/${params.row.contractAddress}`}
          className="text-[#ffa729] hover:text-[#ffb954] transition-colors"
        >
          {params.row.contractAddress}
        </a>
      ),
    },
  ];

  return (
    <ThemeProvider theme={darkTheme}>
      <Box className="p-4">
        <Typography 
          variant="h5" 
          component="h1" 
          className="mb-6 text-center font-bold text-gray-900 dark:text-white"
        >
          Smart Contracts
        </Typography>
        <Box className="h-[600px] w-full">
          {post && 
            <DataGrid
              rows={post}
              columns={columns}
              paginationModel={paginationModel}
              onPaginationModelChange={setPaginationModel}
              pageSizeOptions={[5, 10, 25, 50]}
              checkboxSelection={false}
              disableRowSelectionOnClick
              components={{
                Toolbar: GridToolbar,
              }}
              componentsProps={{
                toolbar: {
                  showQuickFilter: true,
                  quickFilterProps: { debounceMs: 500 },
                },
              }}
              className="dark:bg-secondary-dark rounded-lg shadow-lg"
              sx={{
                border: 'none',
                '& .MuiDataGrid-cell': {
                  borderColor: 'rgba(255, 255, 255, 0.1)',
                },
                '& .MuiDataGrid-columnHeaders': {
                  backgroundColor: 'rgba(0, 0, 0, 0.2)',
                  color: 'white',
                },
                '& .MuiDataGrid-row:hover': {
                  backgroundColor: 'rgba(255, 255, 255, 0.04)',
                },
              }}
            />
          }
        </Box>
      </Box>
    </ThemeProvider>
  );
}
