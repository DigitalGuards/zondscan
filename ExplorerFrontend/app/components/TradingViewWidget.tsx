import React, { useEffect, useRef } from 'react';

declare global {
  interface Window {
    TradingView: any;
  }
}

export default function TradingViewWidget() {
  const container = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const createWidget = () => {
      if (window.TradingView && container.current) {
        new window.TradingView.widget({
          container_id: 'tradingview_qrl',
          width: '100%',
          height: 400,
          symbol: 'MEXC:QRLUSDT',
          interval: 'D',
          timezone: 'Etc/UTC',
          theme: 'dark',
          style: '1',
          locale: 'en',
          toolbar_bg: '#1f1f1f',
          enable_publishing: false,
          hide_side_toolbar: false,
          allow_symbol_change: false,
          studies: [
            'MASimple@tv-basicstudies',
            'RSI@tv-basicstudies'
          ],
          show_popup_button: true,
          popup_width: '1000',
          popup_height: '650'
        });
      }
    };

    if (!document.getElementById('tradingview-script')) {
      const script = document.createElement('script');
      script.id = 'tradingview-script';
      script.src = 'https://s3.tradingview.com/tv.js';
      script.async = true;
      script.onload = createWidget;
      document.head.appendChild(script);

      return () => {
        const scriptElement = document.getElementById('tradingview-script');
        if (scriptElement) {
          scriptElement.remove();
        }
      };
    } else {
      createWidget();
    }
  }, []);

  return (
    <div className="tradingview-widget-container">
      <div 
        id="tradingview_qrl"
        ref={container}
        style={{ height: '400px', width: '100%' }}
      />
    </div>
  );
}
