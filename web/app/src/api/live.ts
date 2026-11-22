import { ORG } from '../config';
import { FIRST_STEP_MS, judge, missedSomething, nextStep, reconnectPause } from './stream';
import type { ConversationPart, LiveOut } from './types';

export interface LiveHandlers {
  onParts(parts: ConversationPart[]): void;
  onConnected(connected: boolean): void;
  /** Loads the conversations view fresh and answers with the position of that load. */
  reloadConversations(): Promise<number>;
  onPiece(piece: { message: string; at: number; text: string }): void;
  onAgentStatus(status: { agent: string; state: string; on: string | null }): void;
  onGone(): void;
  /** `last_sent` confirmed that nothing is missing. */
  onLevel(): void;
}

const listeners = new Set<(message: LiveOut) => void>();

/** Hears every update applied, in order. The feed and the history tabs are applied from here. */
export function subscribeLive(listener: (message: LiveOut) => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

const ASK_PAUSE_MS = 500;

/**
 * The live connection of one screen. It holds the screen's position in the two streams, which only
 * a full load of that stream sets, and applies an update only directly after the last one applied.
 */
export class LiveLink {
  private conversations: number | null = null;
  private feed: number | null = null;
  private edition: number | null = null;
  private socket: WebSocket | null = null;
  private stopped = false;
  private step = FIRST_STEP_MS;
  private retry: ReturnType<typeof setTimeout> | undefined;
  private askTimer: ReturnType<typeof setTimeout> | undefined;
  private askAgain = false;
  private loadingFeed = false;
  /** The feed said it was unavailable: when it is back, it is loaded again and the notice goes. */
  private feedWasAway = false;
  private loadingConversations = false;
  private feedLoader: (() => Promise<void>) | null = null;
  private readonly watching = new Map<string, { conversation: string; thread: string | null }>();

  constructor(
    private readonly person: string,
    private readonly handlers: LiveHandlers,
    private readonly random: () => number = Math.random,
  ) {}

  start(number: number) {
    this.conversations = number;
    this.open();
  }

  stop() {
    this.stopped = true;
    clearTimeout(this.retry);
    clearTimeout(this.askTimer);
    this.socket?.close();
  }

  /* ---------- the feed's position ---------- */

  setFeedLoader(load: (() => Promise<void>) | null) {
    this.feedLoader = load;
  }

  /** Loads the feed as a full load: what it answers becomes the position. */
  loadFeed() {
    if (this.loadingFeed || !this.feedLoader) return;
    this.loadingFeed = true;
    this.feed = null;
    this.edition = null;
    void this.feedLoader().finally(() => {
      this.loadingFeed = false;
    });
  }

  feedAway() {
    this.feedWasAway = true;
  }

  feedLoaded(number: number, edition: number) {
    this.feedWasAway = false;
    this.feed = number;
    this.edition = edition;
    this.ask(); // whatever came while the load was on its way
  }

  /* ---------- watching ---------- */

  watch(conversation: string, thread: string | null, open: boolean) {
    const key = `${conversation}|${thread ?? ''}`;
    if (open === this.watching.has(key)) return;
    if (open) this.watching.set(key, { conversation, thread });
    else this.watching.delete(key);
    this.say({ watching: { conversation, thread, open } });
  }

  /* ---------- the connection ---------- */

  private say(message: unknown) {
    if (this.socket?.readyState === WebSocket.OPEN) this.socket.send(JSON.stringify(message));
  }

  private sendResume() {
    if (this.conversations === null) return;
    this.say({ resume: { conversations: this.conversations, feed: this.feed, feed_edition: this.edition } });
  }

  /** Asks again from our own numbers, at most once per pause: updates in flight would repeat it. */
  private ask() {
    if (this.askTimer !== undefined) {
      this.askAgain = true;
      return;
    }
    this.sendResume();
    this.askTimer = setTimeout(() => {
      this.askTimer = undefined;
      if (this.askAgain) {
        this.askAgain = false;
        this.ask();
      }
    }, ASK_PAUSE_MS);
  }

  private open() {
    if (this.stopped) return;
    const scheme = location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${scheme}://${location.host}/orgs/${encodeURIComponent(ORG)}/live?person=${encodeURIComponent(this.person)}`;
    const ws = new WebSocket(url);
    this.socket = ws;
    ws.onopen = () => {
      this.step = FIRST_STEP_MS;
      this.handlers.onConnected(true);
      this.sendResume();
      for (const { conversation, thread } of this.watching.values()) {
        ws.send(JSON.stringify({ watching: { conversation, thread, open: true } }));
      }
    };
    ws.onmessage = (event) => this.receive(JSON.parse(String(event.data)) as LiveOut);
    ws.onclose = () => {
      if (this.stopped || this.socket !== ws) return;
      this.handlers.onConnected(false);
      this.retry = setTimeout(() => this.open(), reconnectPause(this.step, this.random));
      this.step = nextStep(this.step);
    };
  }

  private needConversations() {
    if (this.loadingConversations) return;
    this.loadingConversations = true;
    this.conversations = null;
    this.handlers
      .reloadConversations()
      .then((number) => {
        this.loadingConversations = false;
        this.conversations = number;
        this.ask();
      })
      .catch(() => {
        this.loadingConversations = false;
        if (!this.stopped) this.retry = setTimeout(() => this.needConversations(), 1000);
      });
  }

  private receive(message: LiveOut) {
    if ('organization_gone' in message) {
      this.stop();
      this.handlers.onGone();
    } else if ('last_sent' in message) {
      this.checkLastSent(message.last_sent);
    } else if ('too_far_behind' in message) {
      if (message.too_far_behind === 'feed') this.loadFeed();
      else this.needConversations();
    } else if ('piece' in message) {
      this.handlers.onPiece(message.piece);
    } else if ('agent_status' in message) {
      this.handlers.onAgentStatus(message.agent_status);
    } else if (message.stream === 'conversations') {
      if (this.conversations === null) return; // being loaded fresh: it is covered, or asked for after
      const verdict = judge(this.conversations, message.after, message.number);
      if (verdict === 'ask') this.ask();
      if (verdict !== 'apply') return;
      this.conversations = message.number;
      this.handlers.onParts(message.parts);
      listeners.forEach((listener) => listener(message));
    } else if (message.stream === 'feed') {
      if (this.edition !== null && message.edition !== this.edition) return this.loadFeed();
      if (this.feed === null) return this.loadFeed();
      const verdict = judge(this.feed, message.after, message.number);
      if (verdict === 'ask') this.ask();
      if (verdict !== 'apply') return;
      this.feed = message.number;
      listeners.forEach((listener) => listener(message));
    }
  }

  private checkLastSent(sent: { conversations: number; feed: number | null; feed_edition: number | null }) {
    let level = this.conversations !== null;
    if (this.conversations !== null && missedSomething(this.conversations, sent.conversations)) {
      level = false;
      this.ask();
    }
    if (sent.feed === null) {
      this.feedWasAway = true;
    } else if (sent.feed_edition !== null) {
      if (this.feedWasAway || (this.edition !== null && sent.feed_edition !== this.edition)) {
        level = false;
        this.loadFeed();
      } else if (this.feed === null) {
        level = false;
        this.loadFeed(); // the feed was unavailable at load and is back
      } else if (missedSomething(this.feed, sent.feed)) {
        level = false;
        this.ask();
      }
    }
    if (level) this.handlers.onLevel();
  }
}
