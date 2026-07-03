"use client";

import React, { useCallback, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import Link from "@tiptap/extension-link";
import Image from "@tiptap/extension-image";
import TextAlign from "@tiptap/extension-text-align";
import { Table } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableCell from "@tiptap/extension-table-cell";
import TableHeader from "@tiptap/extension-table-header";
import { TextStyle } from "@tiptap/extension-text-style";
import Color from "@tiptap/extension-color";
import Highlight from "@tiptap/extension-highlight";
import {
  Bold,
  Italic,
  List,
  ListOrdered,
  Quote,
  Undo,
  Redo,
  Link as LinkIcon,
  Heading1,
  Heading2,
  Heading3,
  Code,
  AlignLeft,
  AlignCenter,
  AlignRight,
  Table as TableIcon,
  FileCode,
} from "lucide-react";
import { Button } from "@/components/ui/button";

export function Composer({
  value,
  onChange,
  placeholder = "Write your message...",
}: {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
}) {
  const t = useTranslations("settings");

  const [isCodeView, setIsCodeView] = useState(false);
  const [codeHtml, setCodeHtml] = useState(value);

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        heading: {
          levels: [1, 2, 3],
        },
        link: false,
      }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { class: "text-primary underline" },
      }),
      TextAlign.configure({
        types: ["heading", "paragraph"],
      }),
      Table.configure({
        resizable: true,
      }),
      TableRow,
      TableCell,
      TableHeader,
      TextStyle,
      Color,
      Highlight.configure({ multicolor: true }),
      Image.configure({
        inline: false,
        allowBase64: true,
      }),
      Placeholder.configure({ placeholder }),
    ],
    [placeholder],
  );

  const editor = useEditor({
    extensions,
    content: value,
    immediatelyRender: false,
    onUpdate: ({ editor }) => {
      onChange(editor.getHTML());
    },
    editorProps: {
      attributes: {
        class:
          "prose prose-sm dark:prose-invert focus:outline-none max-w-none min-h-[150px] px-4 py-3",
      },
      handlePaste: (view, event) => {
        const items = event.clipboardData?.items;
        if (items && items.length > 0) {
          for (let i = 0; i < items.length; i++) {
            const item = items[i];
            if (item.type.startsWith("image/")) {
              event.preventDefault();
              const file = item.getAsFile();
              if (file) {
                const reader = new FileReader();
                reader.onload = () => {
                  if (reader.result && typeof reader.result === "string") {
                    const { state } = view;
                    const node = state.schema.nodes.image?.create({
                      src: reader.result,
                    });
                    if (node) {
                      view.dispatch(state.tr.replaceSelectionWith(node));
                    }
                  }
                };
                reader.readAsDataURL(file);
              }
              return true;
            }
          }
        }
        return false;
      },
    },
  });

  const setContent = useCallback(
    (html: string) => {
      if (editor && html !== editor.getHTML()) {
        editor.commands.setContent(html);
      }
    },
    [editor],
  );

  // Sync external value changes (e.g., when switching to reply mode)
  React.useEffect(() => {
    if (editor && editor.isEditable) {
      setContent(value);
    }
  }, [value, editor, setContent]);

  if (!editor) return null;

  const addLink = () => {
    const previousUrl = editor.getAttributes("link").href;
    const url = window.prompt(t("toolbar_link_url"), previousUrl || "https://");
    if (url === null) return;
    if (url === "") {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
      return;
    }
    editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
  };

  const toggleCodeView = () => {
    if (!isCodeView) {
      // Switching TO code view: capture current editor HTML
      setCodeHtml(editor.getHTML());
    } else {
      // Switching FROM code view: apply textarea content back to editor
      editor.commands.setContent(codeHtml);
      onChange(codeHtml);
    }
    setIsCodeView(!isCodeView);
  };

  return (
    <div className="border border-border-muted rounded-lg overflow-hidden bg-card-bg">
      <div className="flex gap-0.5 px-2 py-1 border-b border-border-muted bg-muted/30 flex-wrap">
        {/* Text formatting */}
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("bold") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleBold().run()}
          title={t("toolbar_bold")}
        >
          <Bold className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("italic") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleItalic().run()}
          title={t("toolbar_italic")}
        >
          <Italic className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("code") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleCode().run()}
          title={t("toolbar_code")}
        >
          <Code className="w-3.5 h-3.5" />
        </Button>
        <div className="w-px h-5 bg-border mx-1" />
        {/* Headings */}
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("heading", { level: 1 }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() =>
            editor.chain().focus().toggleHeading({ level: 1 }).run()
          }
          title={t("toolbar_h1")}
        >
          <Heading1 className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("heading", { level: 2 }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() =>
            editor.chain().focus().toggleHeading({ level: 2 }).run()
          }
          title={t("toolbar_h2")}
        >
          <Heading2 className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("heading", { level: 3 }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() =>
            editor.chain().focus().toggleHeading({ level: 3 }).run()
          }
          title={t("toolbar_h3")}
        >
          <Heading3 className="w-3.5 h-3.5" />
        </Button>
        <div className="w-px h-5 bg-border mx-1" />
        {/* Lists */}
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("bulletList") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          title={t("toolbar_bullet")}
        >
          <List className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("orderedList") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          title={t("toolbar_ordered")}
        >
          <ListOrdered className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("blockquote") ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          title={t("toolbar_quote")}
        >
          <Quote className="w-3.5 h-3.5" />
        </Button>
        <div className="w-px h-5 bg-border mx-1" />
        {/* Text align */}
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive({ textAlign: "left" }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().setTextAlign("left").run()}
          title={t("toolbar_align_left")}
        >
          <AlignLeft className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive({ textAlign: "center" }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().setTextAlign("center").run()}
          title={t("toolbar_center")}
        >
          <AlignCenter className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive({ textAlign: "right" }) ? "bg-muted-foreground/20" : ""}`}
          onClick={() => editor.chain().focus().setTextAlign("right").run()}
          title={t("toolbar_align_right")}
        >
          <AlignRight className="w-3.5 h-3.5" />
        </Button>
        <div className="w-px h-5 bg-border mx-1" />
        {/* Link & Table */}
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${editor.isActive("link") ? "bg-muted-foreground/20" : ""}`}
          onClick={addLink}
          title={t("toolbar_link")}
        >
          <LinkIcon className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0"
          onClick={() =>
            editor
              .chain()
              .focus()
              .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
              .run()
          }
          title={t("toolbar_table")}
        >
          <TableIcon className="w-3.5 h-3.5" />
        </Button>
        <div className="flex-1" />
        <Button
          variant="ghost"
          size="sm"
          className={`h-7 w-7 p-0 ${isCodeView ? "bg-primary/20 text-primary" : ""}`}
          onClick={toggleCodeView}
          title={isCodeView ? "Visual editor" : "HTML source"}
        >
          <FileCode className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0"
          onClick={() => editor.chain().focus().undo().run()}
          title={t("toolbar_undo")}
        >
          <Undo className="w-3.5 h-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className="h-7 w-7 p-0"
          onClick={() => editor.chain().focus().redo().run()}
          title={t("toolbar_redo")}
        >
          <Redo className="w-3.5 h-3.5" />
        </Button>
      </div>
      {isCodeView ? (
        <textarea
          className="w-full min-h-[150px] px-4 py-3 bg-card-bg text-text-main font-mono text-sm focus:outline-none resize-y"
          value={codeHtml}
          onChange={(e) => {
            setCodeHtml(e.target.value);
            onChange(e.target.value);
          }}
          placeholder={placeholder}
          spellCheck={false}
        />
      ) : (
        <EditorContent editor={editor} />
      )}
    </div>
  );
}
