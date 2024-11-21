import * as React from 'react';
import { Buffer } from 'buffer';
import axios from "axios";
import { DataGrid, GridToolbar } from '@mui/x-data-grid';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import Link from "next/link";
import config from '../../config.js';

const Decoder = (params) => {
  const buffer = Buffer.from(params.row.TxHash, 'base64');
  const bufString = buffer.toString('hex');
  const output = "0x" + bufString;
  const url = config.siteUrl + "/tx/" + output;
  return <Link href={url}>{output}</Link>;
};

const DecoderAddress = (params) => {
  const buffer = Buffer.from(params.row.Address, 'base64');
  const bufString = buffer.toString('hex');
  const output = "0x" + bufString;
  const url = config.siteUrl + "/address/" + output;
  return <Link href={url}>{output}</Link>;
};

const ConvertTime = (params) => {
  var timestamp = params.row.TimeStamp;
  var date = new Date(timestamp * 1000);

  return date.getDate()+
  "/"+(date.getMonth()+1)+
  "/"+date.getFullYear()+
  " "+date.getHours()+
  ":"+date.getMinutes()+
  ":"+date.getSeconds();
}

const Divide = (params) => {
  return params.row.Amount / 1000000000;
}

function Transactions() {
  const [post, setPost] = React.useState(null);

  React.useEffect(() => {
    axios.get(config.handlerUrl + "/transactions").then((response) => {
      const data = response.data.response;
  
      const transformedData = data.map((item, index) => ({
        id: index,
        ...item,
        Address: DecoderAddress({row: item}),
        TxHash: Decoder({row: item}),
      }));
  
      setPost(transformedData);
    });
  }, []);

  const columns = [
    {
      field: 'Address', 
      headerName: 'Address', 
      flex: 1,
      valueGetter: (params) => params.row.Address,
      renderCell: (params) => (
        <Link href={config.siteUrl + "/address/" + params.row.Address}>
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
      valueGetter: (params) => params.row.TxHash,
      renderCell: (params) => (
        <Link href={config.siteUrl + "/tx/" + params.row.TxHash}>
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
    <>
      <Box m={2}>
        <Typography variant="h6" component="div" align="center">
          Transactions
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

export default Transactions;
