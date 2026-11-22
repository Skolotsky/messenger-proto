import { ORG } from '../config';
import type { ConversationPart, LiveOut } from './types';

export interface LiveHandlers {
  onParts(parts: ConversationPart[]): void;
  onConnected(connected: boolean): void;
}

/**
 * A minimal live subscription: enough for a sent message to appear on every open screen.
 * Parts are applied by message ID and version, so a repeat is harmless.
 *
 * Seam for T-056: the contract's numbered catch-up (`after`, `last_sent`, `too_far_behind`,
 * the feed stream, pieces of a reply) replaces the handling in `receive` below.
 */
export function connectLive(person: string, startNumber: number, handlers: LiveHandlers): () => void {
  let socket: WebSocket | null = null;
  let stopped = false;
  let delay = 100;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let number = startNumber;

  const receive = (data: string) => {
    const message = JSON.parse(data) as LiveOut;
    if ('stream' in message && message.stream === 'conversations') {
      number = Math.max(number, message.number);
      handlers.onParts(message.parts);
    }
  };

  const open = () => {
    const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${scheme}://${location.host}/orgs/${encodeURIComponent(ORG)}/live?person=${encodeURIComponent(person)}`;
    const ws = new WebSocket(url);
    socket = ws;
    ws.onopen = () => {
      delay = 100;
      handlers.onConnected(true);
      ws.send(JSON.stringify({ resume: { conversations: number, feed: null, feed_edition: null } }));
    };
    ws.onmessage = (event) => receive(String(event.data));
    ws.onclose = () => {
      if (stopped) return;
      handlers.onConnected(false);
      retry = setTimeout(open, delay);
      delay = Math.min(delay * 2, 1000);
    };
  };

  open();
  return () => {
    stopped = true;
    clearTimeout(retry);
    socket?.close();
  };
}
