'use client';

import { useEffect, useRef } from 'react';
import { usePreferences, useResolvedTheme } from './PreferencesProvider';

interface TradingViewNamespace {
  widget: new (config: Record<string, unknown>) => unknown;
}

declare global {
  interface Window {
    TradingView?: TradingViewNamespace;
  }
}

// The script is shared across route visits. Each mounted widget owns its contents
// and load listener, so a late script response cannot recreate an old widget.
export default function TradingViewWidget(): JSX.Element {
  const container = useRef<HTMLDivElement>(null);
  const theme = useResolvedTheme();
  const { preferences, ready } = usePreferences();

  useEffect(() => {
    const target = container.current;
    if (!target || !ready) return;
    const createWidget = () => {
      if (!window.TradingView) return;
      target.replaceChildren();
      new window.TradingView.widget({
        container_id: target.id,
        width: '100%',
        height: 400,
        symbol: 'MEXC:QRLUSDT',
        interval: 'D',
        timezone:
          preferences.timeZone === 'utc'
            ? 'Etc/UTC'
            : Intl.DateTimeFormat().resolvedOptions().timeZone,
        theme: theme === 'light' ? 'light' : 'dark',
        style: '1',
        locale: 'en',
        enable_publishing: false,
        hide_side_toolbar: false,
        allow_symbol_change: false,
        studies: ['MASimple@tv-basicstudies', 'RSI@tv-basicstudies'],
        show_popup_button: true,
        popup_width: '1000',
        popup_height: '650',
      });
    };
    let script = document.getElementById('tradingview-script') as HTMLScriptElement | null;
    if (!script) {
      const newScript = document.createElement('script');
      newScript.id = 'tradingview-script';
      newScript.src = 'https://s3.tradingview.com/tv.js';
      newScript.async = true;
      // Keep failure cleanup attached after unmount so the next route visit
      // can retry even when the request fails while this chart is absent.
      newScript.addEventListener('error', () => newScript.remove(), { once: true });
      document.head.appendChild(newScript);
      script = newScript;
    }
    script.addEventListener('load', createWidget);
    createWidget();
    return () => {
      script.removeEventListener('load', createWidget);
      target.replaceChildren();
    };
  }, [theme, preferences.timeZone, ready]);

  return (
    <div className="tradingview-widget-container">
      <div id="tradingview_qrl" ref={container} style={{ height: 400, width: '100%' }} />
    </div>
  );
}
