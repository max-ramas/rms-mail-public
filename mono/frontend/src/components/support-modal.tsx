"use client";

import { useState, useEffect } from "react";
import { Heart, Copy, Check, Coffee } from "lucide-react";
import { useTranslations } from "next-intl";

interface SupportModalProps {
  isOpen: boolean;
  onClose: () => void;
}

declare global {
  interface Window {
    paypal?: {
      HostedButtons: (options: { hostedButtonId: string }) => {
        render: (selector: string) => void;
      };
    };
  }
}

function PayPalHostedButton() {
  useEffect(() => {
    const containerId = "paypal-container-Z4PFKH33KBQK8";

    const renderButton = () => {
      const container = document.getElementById(containerId);
      if (container) {
        container.innerHTML = ""; // Clear before re-rendering
      }
      if (window.paypal?.HostedButtons) {
        window.paypal
          .HostedButtons({
            hostedButtonId: "Z4PFKH33KBQK8",
          })
          .render("#" + containerId);
      }
    };

    if (!document.getElementById("paypal-sdk-script")) {
      const script = document.createElement("script");
      script.id = "paypal-sdk-script";
      script.src =
        "https://www.paypal.com/sdk/js?client-id=BAAFUa20oWfXXnyoSXswmIKcxpPB1hAjcBRTY6--tRrqfo01S7nP6CDfg43N-TQ43ZnPPhNjHDk2I602mM&components=hosted-buttons&disable-funding=venmo&currency=EUR";
      script.async = true;
      script.onload = renderButton;
      document.body.appendChild(script);
    } else {
      // If script is already loaded (e.g. modal closed and opened again)
      renderButton();
    }
  }, []);

  return (
    <div id="paypal-container-Z4PFKH33KBQK8" className="min-h-[45px]"></div>
  );
}

export function SupportModal({ isOpen, onClose }: SupportModalProps) {
  const [copied, setCopied] = useState(false);
  const t = useTranslations("settings");
  const usdtAddress = "TVoMyT8ecsghioX88iUiunKJrK8QVPJNBh"; // Правильный кошелек

  const handleCopy = () => {
    navigator.clipboard.writeText(usdtAddress);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 backdrop-blur-sm transition-opacity">
      <div className="bg-background w-full max-w-md rounded-xl p-6 shadow-2xl border border-border">
        {/* Хедер */}
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-bold flex items-center gap-2">
            <Heart className="w-5 h-5 text-red-500 fill-red-500" />
            {t("support_modal_title")}
          </h2>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground"
          >
            ✕
          </button>
        </div>

        <p className="text-sm text-muted-foreground mb-4">
          {t("support_modal_desc")}
        </p>

        {/* Провайдер PayPal */}
        <div className="mb-6 bg-muted/30 p-4 rounded-lg border border-border">
          <PayPalHostedButton />
        </div>

        {/* Блок Ko-fi */}
        <div className="mb-6">
          <a
            href="https://ko-fi.com/maxramas"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-center w-full gap-2 bg-[#29abe0] hover:bg-[#228db8] text-white px-4 py-3 rounded-md font-medium transition-colors"
          >
            <Coffee className="w-5 h-5 fill-current" />
            {t("support_modal_kofi")}
          </a>
        </div>

        {/* Разделитель */}
        <div className="relative flex items-center py-2 mb-4">
          <div className="flex-grow border-t border-border"></div>
          <span className="flex-shrink-0 mx-4 text-xs text-muted-foreground uppercase tracking-wider">
            {t("support_modal_or_crypto")}
          </span>
          <div className="flex-grow border-t border-border"></div>
        </div>

        {/* Блок крипты */}
        <div>
          <h3 className="text-sm font-semibold mb-2">
            {t("support_crypto_usdt")}
          </h3>
          <div className="flex items-center gap-2 bg-muted p-2 rounded-md border border-border/50">
            <code className="text-xs flex-1 truncate select-all">
              {usdtAddress}
            </code>
            <button
              onClick={handleCopy}
              className="p-2 bg-background border rounded-md hover:bg-accent transition-colors shrink-0"
              title={t("copy_address")}
            >
              {copied ? (
                <Check className="w-4 h-4 text-green-500" />
              ) : (
                <Copy className="w-4 h-4" />
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
