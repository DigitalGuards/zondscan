// TODO: Fix XMSS address for the explorer

// import React from "react";
// import {Buffer} from 'buffer';
// import {useRouter } from "next/router";
// import TransactionsDisplay from './TransactionsDisplay';
// import Card from 'react-bootstrap/Card';
// import Button from 'react-bootstrap/Button';
// import ListGroup from 'react-bootstrap/ListGroup';
// import WalletGraph from './WalletGraph.jsx';
// import config from '../config.js';
// import Link from "next/link";

// const axios = require('axios');

// // function myFunction() {
// //   // Get the text field
// //   var copyText = document.getElementById("address");

// //   // Select the text field
// //   copyText.select();
// //   copyText.setSelectionRange(0, 99999); // For mobile devices

// //   // Copy the text inside the text field
// //   navigator.clipboard.writeText(copyText.value);
// // }

// function ConvertTime(time){
//   var timestamp = time
//   var date = new Date(timestamp * 1000);

//   return date.getDate()+
//   "/"+(date.getMonth()+1)+
//   "/"+date.getFullYear()+
//   " "+date.getHours()+
//   ":"+date.getMinutes()+
//   ":"+date.getSeconds()
// }

// var formatter = new Intl.NumberFormat('en-US', {
//   style: 'currency',
//   currency: 'USD',
// });

// const cardStyle = {
//   width: "40rem",
//   float: "left",
//   margin: "25px",
//   height: "15rem",
//   backgroundColor: "#031421",
// };


// const cardStyleTwo = {
//   width: "40rem",
//   float: "left",
//   margin: "25px",
//   height: "15rem",
// };

// const cardStyleThree = {
//   color: "white",
//   backgroundColor: "#202528",
//   borderColor: "orange"
// };

// const titleStyle = {
//   margin: "1%",
//   paddingLeft: "20px",
//   color: "white",
//   backgroundColor: "#031421",
// };

// function XMSSAddress() {
//   const search = useLocation();
//   const [address, setAddress] = React.useState(null);
//   const [balance, setBalance] = React.useState(null);
//   const [nonce, setNonce] = React.useState(null);
//   const [total_transactions, setTotalTransactions] = React.useState(null);
//   const [current_price, setCurrentPrice] = React.useState(null);
//   const baseURL = "https://api.coingecko.com/api/v3/coins/quantum-resistant-ledger?tickers=false&market_data=true&community_data=false&developer_data=false&sparkline=false";

//   const [posts, setPosts] = React.useState([]);
//   const [loading, setLoading] = React.useState(false);
//   const [currentPage, setCurrentPage] = React.useState(1);
//   const [postsPerPage] = React.useState(15);
  
//   const [all_transactions, setAllTransactionsByAddress] = React.useState(null);

//   const [first_seen, setFirstSeen] = React.useState(null);
//   const [last_seen, setLastSeen] = React.useState(null);

//   React.useEffect(() => {
//     axios.get(config.handlerUrl + '/address' + search.pathname.slice(7) + "?page=" + currentPage.toString()).then((response) => {
//       console.log(response)
//       const buffer = Buffer.from(response.data.address.id, 'base64');
//       const bufString = buffer.toString('hex');
//       setAddress("0x" + bufString);
//       setBalance(response.data.address.balance / 1000000000);
//       setNonce(response.data.address.nonce);

//       setPosts(response.data.transactions)
//       setTotalTransactions(response.data.total_transactions)
//       setLoading(false);
//     });
//     axios.get(baseURL).then((response) => {
//       setCurrentPrice(formatter.format(response.data.market_data.current_price.usd).substring(1));
//     });
//     axios.get(config.handlerUrl + "/alltransactions/" + search.pathname.slice(13)).then((response) => {
//       setAllTransactionsByAddress(response.data.response)
//       setFirstSeen(ConvertTime(response.data.response[0].TimeStamp))
//       setLastSeen(ConvertTime(response.data.response[(response.data.response.length-1)].TimeStamp))
//     });
//   }, [currentPage]);

//   const paginate = pageNumber => setCurrentPage(pageNumber);

//   const pageNumbers = [];

//   for (let i = 1; i <= Math.ceil(total_transactions / postsPerPage); i++) {
//     pageNumbers.push(i);
//   }

//   return (
//     <>
//     <Button style={{float: "right", marginRight: "50%"}} onClick={() => {navigator.clipboard.writeText(document.getElementById("address").lastChild.data)}} >Copy address</Button>
//     <Card.Title id="address" style={titleStyle}>{address}</Card.Title>
//     {/* <WalletGraph posts={all_transactions}/> */}
//     <Card
//     bg={'Dark'.toLowerCase()}
//     key={'Dark_0'}
//     text={'Dark'.toLowerCase() === 'light' ? 'dark' : 'white'}
//     style={cardStyle}
//     className="mb-2"
//   >
// <Card.Body style={{borderCollapse: "collapse"}}>
//   <Card.Title>Overview</Card.Title>
//   <ListGroup.Item style={cardStyleThree} >Balance: {balance} QRL</ListGroup.Item>
//   <ListGroup.Item style={cardStyleThree} >Value: ${current_price * balance}</ListGroup.Item>
//   <ListGroup.Item style={cardStyleThree} >Nonce: {nonce}</ListGroup.Item>
// </Card.Body>
// </Card>
// <Card
//     bg={'Dark'.toLowerCase()}
//     key={'Dark_1'}
//     text={'Dark'.toLowerCase() === 'light' ? 'dark' : 'white'}
//     style={cardStyleTwo}
//     className="mb-2"
//   >
// <Card.Body>
//   <Card.Title>Wallet information</Card.Title>
//   <Card.Text>
//     This wallet was created on <b>{first_seen}</b> and was last seen on <b>{last_seen}</b> <br/><br/>Current rank is <br/><br/><b>This wallet is <b>{balance > 10000 ? '' : 'not'}</b> qualified to stake on the QRL Blockchain! {balance > 10000 && 'Congratulations!'} <Link href="https://zond-docs.theqrl.org/node/node-staking">Click here to learn more!</Link></b>
//   </Card.Text>
// </Card.Body>
// </Card>
//   <div style={{margin: 0, padding: 0, height: "auto", width: "90%", textAligned: "center", marginLeft: "auto", marginRight: "auto"}}className='transactions'>
//       <TransactionsDisplay posts={posts} loading={loading} />
//       <nav>
//       <ul className='pagination'>
//         {pageNumbers.map(number => (
//           <li key={number} className='page-item'>
//             <Link onClick={() => paginate(number)} href={"#" + number} className='page-link'>
//               {number}
//             </Link>
//           </li>
//         ))}
//       </ul>
//     </nav>
//     </div>
// </>
//   );
// }

// export default XMSSAddress;