'use client';

import * as React from 'react';
import { styled } from '@mui/material/styles';
import Box from '@mui/material/Box';
import Paper from '@mui/material/Paper';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Link from "next/link";
import Image from 'next/image';
import { decodeToHex } from '../lib/helpers';
import { Transaction, TransactionType } from './types';

interface TransactionsDisplayProps {
  posts: Transaction[];
  loading: boolean;
}

const Item = styled(Paper)(({ theme }) => ({
  backgroundColor: theme.palette.mode === 'light' ? '#fff' : theme.palette.background.paper,
  ...theme.typography.body2,
  padding: theme.spacing(1),
  textAlign: 'center',
  color: theme.palette.text.secondary,
}));

function decodeToTxHash(txHash: string): string {
  return decodeToHex(txHash);
}

function convertTime(timestamp: number): string {
  const date = new Date(timestamp * 1000);
  return date.toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });
}

function getTransactionIcon(inOut: number): string {
  return inOut === 0 ? '/receive.svg' : '/send.svg';
}

function formatAmount(amount: number): string {
  return `${amount / 1000000000} QRL`;
}

function getTransactionType(type: TransactionType): string {
  switch (type) {
    case TransactionType.Coinbase:
      return "Coinbase";
    case TransactionType.Attest:
      return "Attest";
    case TransactionType.Transfer:
      return "Transfer";
    case TransactionType.Stake:
      return "Stake";
    default:
      return "Unknown";
  }
}

export default function TransactionsDisplay({ 
  posts, 
  loading 
}: TransactionsDisplayProps): JSX.Element {
  if (loading) {
    return <h2>Loading...</h2>;
  }

  return (
    <Box sx={{ width: '100%' }}>
      <Item>
        <TableHead>
          <TableRow>
            <TableCell>In or Out</TableCell>
            <TableCell>Hash</TableCell>
            <TableCell>Type</TableCell>
            <TableCell>Amount</TableCell>
            <TableCell>Timestamp</TableCell>
          </TableRow>
        </TableHead>
        {posts.map(post => (
          <TableBody key={post.id ?? post.ID}>
            <TableRow>
              <TableCell>
                <Image 
                  width={50}
                  height={50}
                  style={{ display: "inline-block" }} 
                  src={getTransactionIcon(post.InOut)} 
                  alt={post.InOut === 0 ? "Incoming transaction" : "Outgoing transaction"}
                /> 
              </TableCell>
              <TableCell>
                <Link href={`/tx/0x${decodeToTxHash(post.TxHash)}`}>
                  0x{decodeToTxHash(post.TxHash)}
                </Link>
              </TableCell>
              <TableCell>
                {getTransactionType(post.TxType)}
              </TableCell>
              <TableCell>
                {formatAmount(post.Amount)}
              </TableCell>
              <TableCell>
                {convertTime(post.TimeStamp)}
              </TableCell>
            </TableRow>
          </TableBody>
        ))}
      </Item>
    </Box>
  );
}
