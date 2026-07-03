"use client";

import React from "react";
import { useMailInboxPage } from "@/hooks/useMailInboxPage";

export default function HomePage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = React.use(params);
  const { layout } = useMailInboxPage(locale);
  return layout;
}
