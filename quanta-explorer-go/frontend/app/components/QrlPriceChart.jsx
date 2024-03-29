// import React from 'react'
// import Highcharts from 'highcharts'
// import HighchartsReact from 'highcharts-react-official'
// import coingeckodata from './coingecko.csv';
// import * as d3 from 'd3'

// function roundToTwo(num) {
//   return +(Math.round(num + "e+2")  + "e-2");
// }

// export default function MarketcapChart() {
//   const [labels, setLabels] = React.useState(null);
//   const [prices, setPrices] = React.useState(null);

//   React.useEffect(() => {
//     d3.csv(coingeckodata, data => roundToTwo(data.price)).then(setPrices)
//     d3.csv(coingeckodata, data => data.snapped_at.slice(0, 10)).then(setLabels)
//  }, []);

//   const options = {
//   title: {
//     text: 'QRL Price Chart (Updated Weekly)'
//   },
//   chart: {
//     type: 'line',
//     zoomType: 'x'
// },
//   yAxis:{
//     title: {
//       text: 'Dollars'
//     }
//   },
//   xAxis: {
//     categories: labels,
// },
//   series: [{
//     data: prices,
//     name: 'Quantum Resistant Ledger',
//     color: '#fcab5b',
//   }]
// }
//   return (
//     <>
//      <div>
//   <HighchartsReact
//     highcharts={Highcharts}
//     options={options}
//   />
// </div>
//     </>
//      );
// }