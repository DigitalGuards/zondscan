export interface Transaction {
  TxHash: string;
  TimeStamp: number;
  InOut: number;
  Amount: number;
}

export interface TransactionsListProps {
  initialData: {
    txs: Transaction[];
    total: number;
  };
  currentPage: number;
}
