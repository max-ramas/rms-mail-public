"use client";

import React, {
  useState,
  useEffect,
  useRef,
  useMemo,
  forwardRef,
  useImperativeHandle,
} from "react";

export interface EmailBodyHandle {
  getSelectedText: () => string;
}

function emailHoverCursor(el: Element | null): string {
  if (!el) return "";
  if (el.tagName === "IMG" || el.closest("img")) return "zoom-in";
  if (
    el.closest(
      "a[href], button, [role='button'], input[type='button'], input[type='submit'], summary",
    )
  ) {
    return "pointer";
  }
  return "";
}

export const EmailBody = forwardRef<
  EmailBodyHandle,
  {
    html?: string;
    body?: string;
    snippet?: string;
    emailId?: string;
    attachments?: Array<{
      filename: string;
      hash: string;
      size: number;
      content_id?: string;
    }>;
    onBodyClick?: () => void;
  }
>(function EmailBody({ html, body, snippet, emailId, onBodyClick }, ref) {
  const hasHtml = !!html;
  const hasBody = !!body;
  const hasSnippet = !!snippet;

  // Process HTML: add target="_blank" rel="noopener noreferrer" to all <a> tags
  // that don't already have a target — ensures email links open in new tabs
  // without relying on JavaScript interception (which can fail in sandboxed iframes).
  const processedHtml = useMemo(() => {
    if (!html) return html;
    return html.replace(
      /<a\s(?![^>]*\btarget\s*=)/gi,
      '<a target="_blank" rel="noopener noreferrer" ',
    );
  }, [html]);

  // The backend already sanitizes and wraps complex emails. We detect complexity
  // to decide whether to render inline (<div>) or inside an isolated iframe.
  const isComplex = hasHtml
    ? html.toLowerCase().includes("<table") ||
      html.toLowerCase().includes("<style") ||
      html.includes("data-rms-normalized")
    : false;

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const observerRef = useRef<ResizeObserver | null>(null);
  const onBodyClickRef = useRef(onBodyClick);
  onBodyClickRef.current = onBodyClick;
  const [iframeHeight, setIframeHeight] = useState("400px");
  const [lightboxImg, setLightboxImg] = useState<string | null>(null);

  const updateHeight = () => {
    const iframe = iframeRef.current;
    if (iframe?.contentDocument) {
      const wrapper = iframe.contentDocument.getElementById("rms-mail-wrapper");
      if (wrapper) {
        const h = wrapper.scrollHeight;
        if (h > 0) {
          const extra = isComplex ? 0 : 4;
          setIframeHeight(`${h + extra}px`);
        }
      }
    }
  };

  const handleIframeLoad = () => {
    updateHeight();
    const iframe = iframeRef.current;
    const doc = iframe?.contentDocument;
    if (doc) {
      const target = doc.getElementById("rms-mail-wrapper") || doc.body;
      if (target) {
        observerRef.current?.disconnect();
        const observer = new ResizeObserver(() => updateHeight());
        observer.observe(target);
        observerRef.current = observer;
      }

      const imgs = doc.querySelectorAll("img");
      imgs.forEach((img) => {
        img.style.cursor = "zoom-in";
        img.addEventListener("load", updateHeight);
        img.addEventListener("error", updateHeight);
      });

      // Handle cursor style on hover (replaces pointer-events-none approach)
      doc.addEventListener(
        "mousemove",
        (e) => {
          const el = doc.elementFromPoint(e.clientX, e.clientY);
          if (el && doc.body) {
            doc.body.style.cursor = emailHoverCursor(el);
          }
        },
        { passive: true },
      );

      // Handle clicks inside iframe — buttons, images, and other clicks
      // (Links already open in new tabs via target="_blank" in processedHtml)
      doc.addEventListener("click", (e) => {
        const target = e.target as HTMLElement;

        // 1. Buttons / submit inputs — prevent form submission, open form action
        const btn = target.closest(
          "button, input[type='button'], input[type='submit'], [role='button'], summary",
        );
        if (btn) {
          e.preventDefault();
          e.stopPropagation();
          const form = btn.closest("form");
          if (form?.action) {
            window.open(form.action, "_blank");
          }
          return;
        }

        // 2. Images (standalone, not inside a link) — lightbox
        const img =
          target.tagName === "IMG"
            ? (target as HTMLImageElement)
            : target.closest("img");
        if (img?.src) {
          e.preventDefault();
          setLightboxImg(img.src);
          return;
        }

        // 3. Other clicks — let the parent know (e.g. close viewer)
        onBodyClickRef.current?.();
      });

      onBodyClickRef.current = onBodyClick;
    }
  };

  useEffect(() => {
    return () => {
      observerRef.current?.disconnect();
    };
  }, []);

  // Signal to global Escape handler that lightbox is open
  useEffect(() => {
    if (lightboxImg) {
      document.body.setAttribute("data-lightbox-open", "1");
      const handler = () => setLightboxImg(null);
      window.addEventListener("lightbox:close", handler);
      return () => {
        document.body.removeAttribute("data-lightbox-open");
        window.removeEventListener("lightbox:close", handler);
      };
    }
  }, [lightboxImg]);

  useImperativeHandle(
    ref,
    () => ({
      getSelectedText: () => {
        try {
          const iframe = iframeRef.current;
          if (iframe?.contentWindow) {
            const sel = iframe.contentWindow.getSelection();
            if (sel && sel.toString().trim()) {
              return sel.toString().trim();
            }
          }
        } catch {}
        return "";
      },
    }),
    [],
  );

  useEffect(() => {
    if (!hasHtml || !isComplex) return;
    updateHeight();
    const timers = [150, 400, 800, 1500, 3000, 5000].map((d) =>
      setTimeout(updateHeight, d),
    );
    return () => {
      timers.forEach(clearTimeout);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [html, hasHtml, isComplex]);

  if (!hasHtml && !hasBody && !hasSnippet) return null;

  if (hasHtml) {
    if (!isComplex) {
      return (
        <>
          <div
            className="font-sans text-sm leading-relaxed whitespace-pre-wrap wrap-break-word p-4 rounded-xl border border-border/50 bg-card-bg text-foreground email-simple-content"
            dangerouslySetInnerHTML={{ __html: processedHtml || html }}
            onClick={(e) => {
              const target = e.target as HTMLElement;
              // 1. <a> links — open in new tab, prevent navigation
              const link = target.closest<HTMLAnchorElement>("a[href]");
              if (link) {
                e.preventDefault();
                e.stopPropagation();
                window.open(link.getAttribute("href") || link.href, "_blank");
                return;
              }
              // 2. Buttons / submit inputs — prevent form submission, open form action
              const btn = target.closest(
                "button, input[type='button'], input[type='submit'], [role='button'], summary",
              );
              if (btn) {
                e.preventDefault();
                e.stopPropagation();
                const form = btn.closest("form");
                if (form?.action) {
                  window.open(form.action, "_blank");
                }
                return;
              }
              // 3. Images — lightbox
              if (target.tagName === "IMG") {
                const src = (target as HTMLImageElement).src;
                if (src) setLightboxImg(src);
                return;
              }
              // 4. Other clicks
              onBodyClick?.();
            }}
          />
          {lightboxImg && (
            <div
              className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4 cursor-zoom-out backdrop-blur-sm"
              onClick={() => setLightboxImg(null)}
              onKeyDown={(e) => {
                if (e.key === "Escape") setLightboxImg(null);
              }}
              tabIndex={0}
              ref={(el) => el?.focus()}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={lightboxImg}
                className="max-w-full max-h-full object-contain bg-white/5 rounded-lg shadow-2xl"
                alt="Enlarged view"
              />
            </div>
          )}
        </>
      );
    }

    return (
      <>
        <div className="w-full overflow-hidden rounded-xl border border-border/50 bg-white relative">
          <iframe
            key={emailId || "no-email"}
            ref={iframeRef}
            srcDoc={processedHtml || html}
            onLoad={handleIframeLoad}
            sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
            title="Email Content"
            style={{ height: iframeHeight }}
            scrolling="no"
            className="w-full min-w-full m-0 p-0 block border-none"
            tabIndex={-1}
          />
        </div>
        {lightboxImg && (
          <div
            className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 p-4 cursor-zoom-out backdrop-blur-sm"
            onClick={() => setLightboxImg(null)}
            onKeyDown={(e) => {
              if (e.key === "Escape") setLightboxImg(null);
            }}
            tabIndex={0}
            ref={(el) => el?.focus()}
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={lightboxImg}
              className="max-w-full max-h-full object-contain bg-white/5 rounded-lg shadow-2xl"
              alt="Enlarged view"
            />
          </div>
        )}
      </>
    );
  }

  return (
    <div className="font-sans text-sm leading-relaxed whitespace-pre-wrap wrap-break-word p-4 rounded-xl border border-border/50 bg-card-bg text-foreground">
      {body || snippet}
    </div>
  );
});
