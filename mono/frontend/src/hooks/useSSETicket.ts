"use client";

import { useEffect, useRef, useState } from "react";
import { ticketManager } from "@/lib/TicketManager";

type SSECallback = (type: string, event: MessageEvent) => void;

interface UseSSETicketOptions {
  url: string;
  /** List of SSE event types to listen for. */
  events: string[];
  /** Called for each matching event. */
  onEvent: SSECallback;
  /** Whether the connection is enabled. */
  enabled?: boolean;
  /** Called when connection opens (for initial refetch). */
  onOpen?: () => void;
}

export function useSSETicket({
  url,
  events,
  onEvent,
  enabled = true,
  onOpen,
}: UseSSETicketOptions) {
  const esRef = useRef<EventSource | null>(null);
  const mountedRef = useRef(true);
  const onEventRef = useRef(onEvent);
  const onOpenRef = useRef(onOpen);
  // Sync refs on every render so effects always have the latest callbacks
  useEffect(() => {
    onEventRef.current = onEvent;
    onOpenRef.current = onOpen;
  });
  const [reconnectTrigger, setReconnectTrigger] = useState(0);
  const retryCountRef = useRef(0);
  const eventsKey = events.join(",");

  useEffect(() => {
    if (!enabled) return;

    mountedRef.current = true;
    let reconnectTimeout: ReturnType<typeof setTimeout>;
    let cleanupFns: Array<() => void> = [];

    const connect = async () => {
      // Clean up previous connection
      cleanupFns.forEach((fn) => fn());
      cleanupFns = [];
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }

      const ticket = await ticketManager.getTicket();
      if (!mountedRef.current) return;

      const finalUrl = ticket
        ? `${url}${url.includes("?") ? "&" : "?"}ticket=${encodeURIComponent(ticket)}`
        : url;

      const es = new EventSource(finalUrl);
      esRef.current = es;

      es.onopen = () => {
        retryCountRef.current = 0;
        onOpenRef.current?.();
      };

      // Register typed event listeners
      for (const eventType of events) {
        const handler = (e: MessageEvent) => onEventRef.current(eventType, e);
        es.addEventListener(eventType, handler);
        cleanupFns.push(() => es.removeEventListener(eventType, handler));
      }

      es.onerror = () => {
        es.close();
        esRef.current = null;
        if (!mountedRef.current) return;

        // Stop reconnecting after 8 consecutive failures
        if (retryCountRef.current >= 8) {
          if (process.env.NODE_ENV === "development") {
            console.warn("[useSSETicket] Max retries reached, giving up");
          }
          return;
        }

        const delay = Math.min(
          2000 * Math.pow(1.5, retryCountRef.current) + Math.random() * 1000,
          30000,
        );

        reconnectTimeout = setTimeout(() => {
          retryCountRef.current++;
          setReconnectTrigger((c) => c + 1);
        }, delay);
      };
    };

    connect();

    return () => {
      mountedRef.current = false;
      clearTimeout(reconnectTimeout);
      cleanupFns.forEach((fn) => fn());
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, eventsKey, enabled, reconnectTrigger]);
}
