import { 
  Card, 
  CardContent, 
  Typography, 
  Table, 
  TableBody, 
  TableCell, 
  TableContainer, 
  TableHead, 
  TableRow, 
  Button 
} from '@mui/material';

interface Transaction {
  hash: string;
  from: string;
  to: string;
  value: string;
}

interface TransactionCardProps {
  transactions: Transaction[];
  title: string;
  buttonLabel: string;
}

export default function TransactionCard({ 
  transactions, 
  title, 
  buttonLabel 
}: TransactionCardProps): JSX.Element {
  return (
    <Card sx={{ 
      maxWidth: "auto", 
      height: 'auto', 
      marginBottom: 2, 
      marginLeft: 2, 
      marginRight: 2 
    }}>
      <CardContent>
        <Typography variant="h5" component="div">
          {title}
        </Typography>
        <TableContainer>
          <Table size="small" aria-label="transactions table">
            <TableHead>
              <TableRow>
                <TableCell>Hash</TableCell>
                <TableCell>From</TableCell>
                <TableCell>To</TableCell>
                <TableCell>Value (ETH)</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map((transaction) => (
                <TableRow key={transaction.hash}>
                  <TableCell>{transaction.hash}</TableCell>
                  <TableCell>{transaction.from}</TableCell>
                  <TableCell>{transaction.to}</TableCell>
                  <TableCell>{transaction.value}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </CardContent>
      <Button 
        sx={{ width: "100%" }} 
        variant="contained" 
        color="primary" 
        onClick={() => {}}
      >
        {buttonLabel}
      </Button>
    </Card>
  );
}
