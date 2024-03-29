import React from "react";
import Divider from '@mui/material/Divider';
import config from '../config.js';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';

const axios = require('axios');

function createData(slotnumber, leader, attestors) {
  return { slotnumber, leader, attestors };
}

function Validators() {
  const [validatorsbyslotnumber, setValidatorsBySlotNumber] = React.useState(null);
  const [epoch, setEpoch] = React.useState(null);
  React.useEffect(() => {
    axios.get(config.handlerUrl + '/validators').then((response) => {
      setValidatorsBySlotNumber(response.data.response.resultvalidator.validatorsbyslotnumber)
      setEpoch(response.data.response.resultvalidator.epoch);
    });
  }, []);

  console.log(validatorsbyslotnumber);

  const rows = [
  ];
  
  const data = validatorsbyslotnumber?.map((item, index) => rows.push(createData(item.slotnumber, item.leader, item.attestors?.map((a, i)=> a + "\n"))));
  return (
//     <>
//     <h3 style={{textAlign: "center"}}>Epoch: {epoch}</h3>
//    <Card style={{width: "75rem", backgroundColor: "#031421"}}>
//     <div class="scrollable">
//         <ListGroup>
// </ListGroup>
//   {listItems}
//     </div>
//     </Card>
//     </>

<>
<Divider />
<Typography variant="h6" component="div" m={2} align="center">
  Validators
</Typography>
<Divider />
<TableContainer component={Paper}>
<Table sx={{ width: "75%", margin: "auto"}}>
  <TableHead>
    <TableRow>
      <TableCell>Slot Number</TableCell>
      <TableCell align="right">Block Proposer</TableCell>
      <TableCell align="right">Attestors</TableCell>
    </TableRow>
  </TableHead>
  <TableBody>
    {rows.map((row) => (
      <TableRow
        key={row.name}
        sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
      >
        <TableCell align="left">{row.slotnumber}</TableCell>
        <TableCell align="right">{row.leader}</TableCell>
        <TableCell align="right">{row.attestors}</TableCell>
      </TableRow>
    ))}
  </TableBody>
</Table>
</TableContainer>
</>
  );
}

export default Validators;
