import React from "react";
import type { Metadata, Viewport } from "next";
import Script from "next/script";
import { cookies, headers } from "next/headers";
import "./globals.css";

const RTL_LOCALES = new Set(["ar", "he", "ur"]);

const EDITION_SUFFIX = (() => {
  const e = (process.env.NEXT_PUBLIC_EDITION || "unified").toLowerCase();
  if (e === "mono_pro" || e === "monopro") return "\u00A0MP";
  if (e.startsWith("m")) return "\u00A0M";
  if (e.startsWith("t")) return "\u00A0T";
  return "\u00A0U";
})();

export const metadata: Metadata = {
  title: `RMS\u00A0Mail${EDITION_SUFFIX}`,
  description: "High-performance email client with AI",
  appleWebApp: {
    capable: true,
    title: `RMS\u00A0Mail${EDITION_SUFFIX}`,
    statusBarStyle: "default",
  },
};

export const viewport: Viewport = {
  themeColor: "#f59e0b",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Determine active locale from next-intl header or cookie
  const headerList = await headers();
  const cookieStore = await cookies();

  let locale =
    headerList.get("x-next-intl-locale") ||
    cookieStore.get("NEXT_LOCALE")?.value ||
    "en";

  // Extract locale from pathname as fallback
  if (!locale || locale === "en") {
    const pathname =
      headerList.get("x-invoke-path") || headerList.get("x-matched-path") || "";
    const match = pathname.match(/^\/([a-z]{2})(\/|$)/);
    if (match) {
      locale = match[1];
    }
  }

  const dir = RTL_LOCALES.has(locale) ? "rtl" : "ltr";

  return (
    <html lang={locale} dir={dir} suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        {/* eslint-disable-next-line @next/next/no-page-custom-font */}
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap"
          rel="stylesheet"
        />
        <Script
          id="theme-init"
          strategy="beforeInteractive"
        >{`(function(){try{var d=true;var c=document.cookie.match(/(?:^|; )theme=(dark|light)(?:;|$)/);if(c){d=c[1]==="dark"}else{var t=localStorage.getItem("rms-mail_theme");if(t)d=t==="dark"}document.documentElement.classList.toggle("dark",d)}catch(e){}})()`}</Script>
        <Script
          id="sw-register"
          strategy="afterInteractive"
        >{`if('serviceWorker' in navigator){if(${process.env.NODE_ENV === "production" ? "true" : "false"}){navigator.serviceWorker.register('/sw.js')}else{navigator.serviceWorker.getRegistrations().then(function(rs){rs.forEach(function(r){r.unregister()})})}}`}</Script>
      </head>
      <body className="font-sans antialiased">{children}</body>
    </html>
  );
}
