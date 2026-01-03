import React, { useState } from 'react';
import axios from "axios";
import config from '../../config.js';
import BrushChart from './BlockSizeChart';
import Typography from '@mui/material/Typography';
import Divider from '@mui/material/Divider';

const NewBlockSizeChart = (): JSX.Element => {
    const [loading, setLoading] = useState(true);
    const [blocks, setBlocks] = useState([]);

    React.useEffect(() => {
        axios.get(config.handlerUrl + "/blocksizes").then((response) => {
            setBlocks(response.data.response);
        }).finally(() => setLoading(false));
    }, []);

    console.log(loading);
    console.log(blocks);

    return (
        <>
            <Divider />
            <Typography variant="h6" component="div" m={2} align="center">
                Average Block Size Chart
                {loading ? <div>Loading....</div> : <BrushChart width={750} height={500} blocks={blocks} />}
            </Typography>
        </>
    );
}

export default NewBlockSizeChart;