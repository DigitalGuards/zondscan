// import React from 'react'
// import Highcharts from "highcharts/highstock";
// import HighchartsReact from 'highcharts-react-official'

// function ConvertTime(time){
//     var timestamp = time
//     var date = new Date(timestamp * 1000);

//     return (date.getMonth()+1)+
//     "/"+date.getFullYear()
// }

// const WalletGraph = ({ posts }) => {
//   let amount_of_quanta = [0];
//   let dates = [];

//   let graph_value = 0;

//   posts?.map(item => {if (item.InOut == 1){ (graph_value += (item.Amount / 1000000000))} else{ graph_value -= (-Math.abs(item.Amount / 1000000000))} (amount_of_quanta.push(graph_value))});
//   posts?.map(item => dates.push(ConvertTime(item.TimeStamp)));

//   const options = {
//     rangeSelector: {
//         buttons: [{
//           type: 'month',
//           count: 1,
//           text: '1m',
//         }, {
//           type: 'month',
//           count: 3,
//           text: '3m'
//         }, {
//           type: 'month',
//           count: 6,
//           text: '6m'
//         }, {
//           type: 'ytd',
//           text: 'YTD'
//         }, {
//           type: 'year',
//           count: 1,
//           text: '1y'
//         }, {
//           type: 'all',
//           text: 'All'
//         }]
//       },
//   title: {
//     text: 'Quanta Chart'
//   },
//   chart: {
//     width: 1400,
//     height: 400,
//     backgroundColor: {
//         linearGradient: { x1: 0, y1: 0, x2: 1, y2: 1 },
//         stops: [
//             [0, '#2a2a2b'],
//             [1, '#3e3e40']
//         ]
//     },
//     type: 'line',
//     zoomType: 'x'
// },
//   yAxis:{
//     opposite: false,
//     title: {
//       text: 'Amount of Quanta'
//     }
//   },
//   xAxis: {
//     categories: dates,
// },
//   series: [{
//     data: amount_of_quanta,
//     name: 'QRL',
//     color: '#fcab5b',
//   }],
// }
//   return (
//     <>
//      <div>
//     <center><HighchartsReact
//     highcharts={Highcharts}
//     constructorType={'stockChart'}
//     options={options}
//   /></center>
// </div>
//     </>
//      );
// }

// export default WalletGraph;