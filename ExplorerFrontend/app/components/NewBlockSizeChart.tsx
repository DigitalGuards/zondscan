import React, { useState, useRef, useEffect } from 'react';
import axios from "axios";
import config from '../../config.js';
import BrushChart from './BlockSizeChart';

const NewBlockSizeChart = (): JSX.Element => {
    const [loading, setLoading] = useState(true);
    const [blocks, setBlocks] = useState([]);
    const containerRef = useRef<HTMLDivElement>(null);
    const [width, setWidth] = useState(600);

    React.useEffect(() => {
        axios.get(config.handlerUrl + "/blocksizes").then((response) => {
            setBlocks(response.data.response);
        }).finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        if (!containerRef.current) return;
        const ro = new ResizeObserver(([entry]) => setWidth(Math.floor(entry.contentRect.width)));
        ro.observe(containerRef.current);
        return () => ro.disconnect();
    }, []);

    return (
        <>
            <hr className="border-[#3d3d3d] my-4" />
            <div ref={containerRef}>
                <h2 className="text-lg font-semibold text-white text-center mb-2">
                    Average Block Size Chart
                </h2>
                {loading ? <div>Loading....</div> : <BrushChart width={width} height={500} blocks={blocks} />}
            </div>
        </>
    );
}

export default NewBlockSizeChart;
