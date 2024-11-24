import { Transaction, InternalTransaction } from './types';

export interface DownloadBtnProps {
  data: Transaction[];
  fileName: string;
}

export interface DownloadBtnInternalProps {
  data: InternalTransaction[];
  fileName: string;
}

export function DownloadBtn(props: DownloadBtnProps): JSX.Element;
export function DownloadBtnInternal(props: DownloadBtnInternalProps): JSX.Element;
