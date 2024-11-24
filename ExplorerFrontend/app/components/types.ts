export interface Transaction {
  ID: string;
  InOut: number;
  TxType: number;
  Address: string;
  TransactionAddress: string;
  TxHash: string;
  TimeStamp: number;
  Amount: number;
  Paidfees: number;
}

export interface InternalTransaction {
  Type: number;
  CallType: string;
  Hash: string;
  From: string;
  To: string;
  Input: string;
  Output: number;
  TraceAddress: Array<number | string>;
  Value: number;
  Gas: number;
  GasUsed: number;
  AddressFunctionIdentifier: string;
  AmountFunctionIdentifier: number;
  BlockTimestamp: number;
}

export interface DownloadBtnProps {
  data: Transaction[];
  fileName: string;
}

export interface DownloadBtnInternalProps {
  data: InternalTransaction[];
  fileName: string;
}
