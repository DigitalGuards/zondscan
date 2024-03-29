"use client"

import * as React from 'react';
import { Buffer } from 'buffer';
import axios from "axios";
import { DataGrid, GridToolbar } from '@mui/x-data-grid';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import config from '../../config.js';
import Link from 'next/link.js';

const DecoderAddress = (params) => {
  const buffer = Buffer.from(params.row.from, 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

const DecoderTxHash = (params) => {
  const buffer = Buffer.from(params.row.txHash, 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

const Decoder = (params) => {
  const buffer = Buffer.from(params.row.pk || params.row.signature, 'base64');
  const bufString = buffer.toString('hex');
  return "0x" + bufString;
};

export default function Contracts() {
  const [post, setPost] = React.useState(null);

  React.useEffect(() => {
    axios.get(config.handlerUrl + "/contracts").then((response) => {
      const data = response.data.response;
  
      const transformedData = data.map((item, index) => ({
        id: index,
        ...item,
        from: DecoderAddress({row: item}),
        txHash: DecoderTxHash({row: item}),
        pk: Decoder({row: item, key: 'pk'}),
        signature: Decoder({row: item, key: 'signature'}),
        contractAddress: DecoderAddress({row: item, key: 'contractAddress'})
      }));
  
      setPost(transformedData);
    });
  }, []);

  const columns = [
    {
      field: 'from', 
      headerName: 'From (Contract Creator Address)', 
      flex: 1,
      renderCell: (params: string) => (
        <Link href={config.siteUrl + "/address/" + params.row.from}>
          {params.row.from}
        </Link>
      ),
    },
    { 
      field: 'txHash', 
      headerName: 'txHash', 
      flex: 1, 
      renderCell: (params: string) => (
        <Link href={config.siteUrl + "/tx/" + params.row.txHash}>
          {params.row.txHash}
        </Link>
      ),
    },
    { 
      field: 'pk', 
      headerName: 'Pk', 
      flex: 1 
    },
    { 
      field: 'signature', 
      headerName: 'Signature', 
      flex: 1 
    },
    { 
      field: 'nonce', 
      headerName: 'Nonce', 
      flex: 1 
    },
    { 
      field: 'value', 
      headerName: 'Value', 
      flex: 1 
    },
    { 
      field: 'contractAddress', 
      headerName: 'Contract Address', 
      flex: 1,
      renderCell: (params: string) => (
        <Link href={config.siteUrl + "/address/" + params.row.contractAddress}>
          {params.row.contractAddress}
        </Link>
      ),
    },
  ];

  return (
    <>
      <Box m={2}>
        <Typography variant="h6" component="div" align="center">
          Contracts
        </Typography>
        <Box height={500} width='100%'>
          {post && 
            <DataGrid
              rows={post}
              columns={columns}
              pageSize={10}
              rowHeight={50} 
              rowsPerPageOptions={[5, 10, 25]}
              components={{
                Toolbar: GridToolbar,
              }}
            />
          }
        </Box>
      </Box>
    </>
  );
}
