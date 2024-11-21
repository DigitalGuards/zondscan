import {Buffer} from 'buffer';
import receive from './../images/receive.svg';
import send from './../images/send.svg';
import * as React from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import { styled } from '@mui/material/styles';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Paper from '@mui/material/Paper';
import Link from "next/link";
import Image from 'next/image'

function DecodeToTxHash(tx_hash){
    const buffer = Buffer.from(tx_hash, 'base64');
    const bufString = buffer.toString('hex');

    return bufString
}

function ConvertTime(time){
    var timestamp = time
    var date = new Date(timestamp * 1000);

    return date.getDate()+
    "/"+(date.getMonth()+1)+
    "/"+date.getFullYear()+
    " "+date.getHours()+
    ":"+date.getMinutes()+
    ":"+date.getSeconds()
}

const Item = styled(Paper)(({ theme }) => ({
  backgroundColor: theme.palette.mode === '#fff',
  ...theme.typography.body2,
  padding: theme.spacing(1),
  textAlign: 'center',
  color: theme.palette.text.secondary,
}));

function InOut(inOut){
    if (inOut == 0){
        return receive
    } else if (inOut == 1){
        return send
    }
}

function Divide(amount){
    return amount / 1000000000;
}

function TypeTransfer(type_transfer){
    if (type_transfer == 0){
        return "Coinbase"
    } else if (type_transfer == 1){
        return "Attest"
    } else if (type_transfer == 2){
        return "Transfer"
    } else if (type_transfer == 3){
        return "Stake"
    }
}

const TransactionsDisplay = ({ posts, loading }) => {
  if (loading) {
    return <h2>Loading...</h2>;
  }

  return (
    <>
    <Box sx={{ width: '100%' }}>
    <Item>
    <TableHead>
    <TableRow>
          <th>In or Out</th>
          <th>Hash</th>
          <th>Type</th>
          <th>Amount</th>
          <th>Timestamp</th>
          </TableRow>
        </TableHead>
      {posts.map(post => (
        <TableBody key={post.id}>
      <TableRow>
        <TableCell>
          {<Image style={{ width: 50, height: 50, display: "inline-block"}} src={InOut(post.InOut)} alt="In or out transaction" />} 
        </TableCell>
        <TableCell>
        <Link href={`/tx/0x${DecodeToTxHash(post.TxHash)}`}>0x{DecodeToTxHash(post.TxHash)}</Link>
        </TableCell>
        <TableCell>
        {TypeTransfer(post.TxType)}
        </TableCell>
        <TableCell>
        {Divide(post.Amount)} QRL
        </TableCell>
        <TableCell>
        {ConvertTime(post.TimeStamp)}
        </TableCell>
        </TableRow>
        </TableBody>
      ))}
    </Item>
    </Box>
    </>);
};

export default TransactionsDisplay;
