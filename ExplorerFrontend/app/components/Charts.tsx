import React from 'react';
import Card from '@mui/material/Card';
import CardContent from '@mui/material/CardContent';
import Typography from '@mui/material/Typography';
import Box from '@mui/material/Box';
import TradingViewWidget from './TradingViewWidget';

export default function Charts() {
  return (
    <Box sx={{ width: '100%', mb: 4 }}>
      <Card sx={{
        backgroundColor: '#1f1f1f',
        border: '1px solid #3d3d3d',
        borderRadius: '16px',
        '&:hover': {
          borderColor: '#ffa729',
        },
        transition: 'border-color 0.3s ease'
      }}>
        <CardContent>
          <Typography 
            variant="h3" 
            gutterBottom 
            sx={{ 
              color: '#ffa729',
              fontSize: { xs: '0.875rem', sm: '1.25rem' },
              fontWeight: 'bold'
            }}
          >
            MEXC QRL/USDT Chart
          </Typography>
          <Box sx={{ height: 400, mt: 2 }}>
            <TradingViewWidget />
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
