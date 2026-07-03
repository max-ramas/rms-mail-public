import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  const suffix = (() => {
    const e = (process.env.NEXT_PUBLIC_EDITION || "unified").toLowerCase();
    if (e.startsWith("m")) return "\u00A0M";
    if (e.startsWith("t")) return "\u00A0T";
    return "\u00A0U";
  })();
  const appName = `RMS\u00A0Mail${suffix}`;
  return {
    name: appName,
    short_name: appName,
    description: "Modern webmail client",
    start_url: "/",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#f59e0b",
    icons: [
      {
        src: "/icon-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: "/icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
    ],
  };
}
