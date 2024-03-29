"use client";

import React, { useState } from 'react';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import TextField from '@mui/material/TextField';
import Alert from '@mui/material/Alert';
import { toFixed } from '../lib/helpers';

function Converter() {
    const [quanta, setQuanta] = useState("");
    const [shor, setShor] = useState("");
    const [error, setError] = useState("");

    const handleChangeShors = (e: any) => {
        const value = e.target.value;
        if (isNaN(value)) {
            setError("Invalid Input: Enter a number");
        } else {
            setError('');
            setQuanta(toFixed((Number(value) / 1e18)).toString());
            setShor(value);
        }
    };

    const handleChangeQuanta = (e: any) => {
        const value = e.target.value;
        if (isNaN(value)) {
            setError("Invalid Input: Enter a number");
        } else {
            setError('');
            setShor(toFixed((Number(value) * 1e18)).toString());
            setQuanta(value);
        }
    };

    return (
        <>
            <Box
                display="flex"
                justifyContent="center"
                alignItems="center"
                flexDirection="column"
                component="form"
                sx={{
                    '& .MuiTextField-root': { m: 1, width: '25ch' },
                }}
                noValidate
                autoComplete="off"
            >
                <Typography variant="h6" component="div" mb={2}>
                    Unit Converter for the QRL
                </Typography>
                <TextField
                    label="Quanta"
                    type="text"
                    value={quanta}
                    onChange={handleChangeQuanta}
                    variant="outlined"
                />
                <TextField
                    label="Shor"
                    type="text"
                    value={shor}
                    onChange={handleChangeShors}
                    variant="outlined"
                />
                {error && (
                    <Alert severity="error" sx={{ width: '50%', mb: 2 }}>
                        {error}
                    </Alert>
                )}
            </Box>
        </>
    );
}

export default Converter;
