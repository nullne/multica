import type { WSMessage, WSEventType } from "@/shared/types";
import { type Logger, noopLogger } from "@/shared/logger";

type EventHandler = (payload: unknown) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private token: string | null = null;
  // Optional getter for the latest access token. Used on every (re)connect so
  // reconnections pick up a token refreshed since the initial connect.
  private tokenProvider: (() => string | null) | null = null;
  private workspaceId: string | null = null;
  private handlers = new Map<WSEventType, Set<EventHandler>>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private hasConnectedBefore = false;
  private onReconnectCallbacks = new Set<() => void>();
  private anyHandlers = new Set<(msg: WSMessage) => void>();
  private logger: Logger;

  constructor(url: string, options?: { logger?: Logger }) {
    this.baseUrl = url;
    this.logger = options?.logger ?? noopLogger;
  }

  setAuth(token: string | (() => string | null), workspaceId: string) {
    if (typeof token === "function") {
      this.tokenProvider = token;
      this.token = token();
    } else {
      this.token = token;
    }
    this.workspaceId = workspaceId;
  }

  connect() {
    const token = this.tokenProvider ? this.tokenProvider() : this.token;
    const url = new URL(this.baseUrl);
    if (token) url.searchParams.set("token", token);
    if (this.workspaceId)
      url.searchParams.set("workspace_id", this.workspaceId);

    this.ws = new WebSocket(url.toString());

    this.ws.onopen = () => {
      this.logger.info("connected");
      if (this.hasConnectedBefore) {
        for (const cb of this.onReconnectCallbacks) {
          try {
            cb();
          } catch {
            // ignore reconnect callback errors
          }
        }
      }
      this.hasConnectedBefore = true;
    };

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data as string) as WSMessage;
      this.logger.debug("received", msg.type);
      const eventHandlers = this.handlers.get(msg.type);
      if (eventHandlers) {
        for (const handler of eventHandlers) {
          handler(msg.payload);
        }
      }
      for (const handler of this.anyHandlers) {
        handler(msg);
      }
    };

    this.ws.onclose = () => {
      this.logger.warn("disconnected, reconnecting in 3s");
      this.reconnectTimer = setTimeout(() => this.connect(), 3000);
    };

    this.ws.onerror = () => {
      // Suppress — onclose handles reconnect; errors during StrictMode
      // double-fire are expected in dev and harmless.
    };
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      // Remove handlers before close to prevent onclose from scheduling a reconnect
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
      this.ws = null;
    }
    this.hasConnectedBefore = false;
    this.handlers.clear();
    this.anyHandlers.clear();
    this.onReconnectCallbacks.clear();
  }

  on(event: WSEventType, handler: EventHandler) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set());
    }
    this.handlers.get(event)!.add(handler);
    return () => {
      this.handlers.get(event)?.delete(handler);
    };
  }

  onAny(handler: (msg: WSMessage) => void) {
    this.anyHandlers.add(handler);
    return () => {
      this.anyHandlers.delete(handler);
    };
  }

  onReconnect(callback: () => void) {
    this.onReconnectCallbacks.add(callback);
    return () => {
      this.onReconnectCallbacks.delete(callback);
    };
  }

  send(message: WSMessage) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }
}
