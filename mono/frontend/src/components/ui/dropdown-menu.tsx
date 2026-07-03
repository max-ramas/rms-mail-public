"use client";

import * as React from "react";
import { createPortal } from "react-dom";

export interface DropdownMenuProps {
  trigger?: React.ReactNode;
  children?: React.ReactNode;
}

export function DropdownMenu({ trigger, children }: DropdownMenuProps) {
  const [open, setOpen] = React.useState(false);
  const triggerRef = React.useRef<HTMLDivElement>(null);
  const [pos, setPos] = React.useState({ top: 0, left: 0 });

  React.useEffect(() => {
    if (!open || !triggerRef.current) return;
    const rect = triggerRef.current.getBoundingClientRect();
    setPos({ top: rect.bottom + 4, left: rect.right - 192 }); // 192px = w-48
  }, [open]);

  React.useEffect(() => {
    if (!open) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        // Also check if click is inside the portal
        const portal = document.getElementById("dropdown-portal");
        if (portal && !portal.contains(e.target as Node)) {
          setOpen(false);
        }
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  const dropdown = open && (
    <div
      id="dropdown-portal"
      className="fixed w-48 bg-card border border-border rounded-md shadow-lg z-[9999]"
      style={{ top: pos.top, left: pos.left }}
    >
      {children}
    </div>
  );

  return (
    <>
      <div ref={triggerRef} onClick={() => setOpen(!open)}>
        {trigger}
      </div>
      {typeof document !== "undefined" && createPortal(dropdown, document.body)}
    </>
  );
}

export interface DropdownMenuItemProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  onClick?: () => void;
  destructive?: boolean;
}

export function DropdownMenuItem({
  children,
  onClick,
  className,
  ...props
}: DropdownMenuItemProps) {
  return (
    <div
      className={`px-4 py-2 text-sm text-foreground hover:bg-muted cursor-pointer ${className || ""}`}
      onClick={onClick}
      {...props}
    >
      {children}
    </div>
  );
}

export type DropdownMenuSeparatorProps = React.HTMLAttributes<HTMLDivElement>;

export function DropdownMenuSeparator({
  className,
  ...props
}: DropdownMenuSeparatorProps) {
  return (
    <div className={`h-px bg-border my-1 ${className || ""}`} {...props} />
  );
}
