import React from "react";
import type { Metadata } from "next";
import { NextIntlClientProvider } from "next-intl";
import { getMessages } from "next-intl/server";
import { notFound } from "next/navigation";
import { Providers } from "@/components/providers";
import { AuthGuard } from "@/components/auth-guard";
import { GlobalMailSSE } from "@/components/global-mail-sse";

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
};

export default async function RootLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const resolvedParams = await params;
  const { locale } = resolvedParams;

  const allLocales = [
    "en",
    "ru",
    "ka",
    "zh",
    "es",
    "hi",
    "ar",
    "fr",
    "pt",
    "ja",
    "ko",
    "de",
    "it",
    "nl",
    "sv",
    "da",
    "nb",
    "fi",
    "lt",
    "lv",
    "et",
    "pl",
    "cs",
    "hu",
    "ro",
    "bg",
    "el",
    "sr",
    "hr",
    "uk",
    "kk",
    "hy",
    "az",
    "uz",
    "tr",
    "id",
    "vi",
    "th",
    "he",
    "ur",
    "bn",
    "ca",
    "ms",
    "sl",
    "sk",
  ];
  if (!allLocales.includes(locale)) {
    notFound();
  }

  const messages = await getMessages();

  return (
    <NextIntlClientProvider locale={locale} messages={messages}>
      <Providers>
        <AuthGuard>
          <GlobalMailSSE />
          {children}
        </AuthGuard>
      </Providers>
    </NextIntlClientProvider>
  );
}
