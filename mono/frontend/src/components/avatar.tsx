"use client";

import { useState } from "react";

export function stringToHslColor(str: string): string {
  let hash = 0;
  const s = str || "";
  for (let i = 0; i < s.length; i++) {
    hash = s.charCodeAt(i) + ((hash << 5) - hash);
  }
  const h = Math.abs(hash) % 360;
  return `hsl(${h}, 55%, 45%)`;
}

export function getInitials(name: string, email: string): string {
  const parts = (name || "").trim().split(/\s+/);
  if (parts.length >= 2 && parts[0] && parts[1]) {
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }
  if (parts.length === 1 && parts[0]) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (email || "?").slice(0, 2).toUpperCase();
}

export function Avatar({
  src,
  name,
  email,
  size = 24,
}: {
  src?: string | null;
  name?: string;
  email: string;
  size?: number;
}) {
  const [prevSrc, setPrevSrc] = useState(src);
  const [error, setError] = useState(false);

  if (src !== prevSrc) {
    setPrevSrc(src);
    setError(false);
  }

  if (src && !error) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={src}
        alt=""
        width={size}
        height={size}
        className="rounded-full shrink-0"
        style={{ width: size, height: size, objectFit: "cover" }}
        onError={() => setError(true)}
      />
    );
  }

  return (
    <div
      className="rounded-full shrink-0 flex items-center justify-center text-white font-bold overflow-hidden"
      style={{
        width: size,
        height: size,
        fontSize: size * 0.4,
        backgroundColor: stringToHslColor(email),
      }}
    >
      {getInitials(name || "", email)}
    </div>
  );
}
