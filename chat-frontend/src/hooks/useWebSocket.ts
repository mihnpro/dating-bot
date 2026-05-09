import { useEffect, useRef, useCallback } from 'react';
import type { WSServerEvent, WSClientMessage } from '../types/chat';

type EventHandler = (event: WSServerEvent) => void;

const WS_URL = (token: string) => {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = import.meta.env.PROD
    ? window.location.host
    : window.location.hostname + ':8083';
  return `${proto}://${host}/ws?token=${encodeURIComponent(token)}`;
};

export function useWebSocket(token: string | null, onEvent: EventHandler) {
  const wsRef = useRef<WebSocket | null>(null);
  const pingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (!token) return;

    const ws = new WebSocket(WS_URL(token));
    wsRef.current = ws;

    ws.onopen = () => {
      pingRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'ping' }));
        }
      }, 30_000);
    };

    ws.onmessage = (evt) => {
      try {
        const data = JSON.parse(evt.data as string) as WSServerEvent;
        onEventRef.current(data);
      } catch {
        // ignore malformed frames
      }
    };

    ws.onerror = () => {
      console.error('[WS] connection error');
    };

    return () => {
      if (pingRef.current) clearInterval(pingRef.current);
      ws.close();
    };
  }, [token]);

  const send = useCallback((msg: WSClientMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  return { send };
}
