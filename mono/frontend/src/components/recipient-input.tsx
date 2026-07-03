"use client";

import React, { useState, useRef, useEffect, KeyboardEvent } from "react";
import { X, User, Mail } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useContacts } from "@/hooks/useEmailQueries";
import { useTranslations } from "next-intl";

export interface Contact {
  id?: string;
  name: string;
  address: string;
  phone?: string;
  notes?: string;
}

interface RecipientInputProps {
  value: string[];
  onChange: (emails: string[]) => void;
  placeholder?: string;
  accountId?: string;
  id?: string;
}

export function RecipientInput({
  value,
  onChange,
  placeholder,
  accountId,
  id,
}: RecipientInputProps) {
  const t = useTranslations("mail");
  const [inputValue, setInputValue] = useState("");
  const [isFocused, setIsFocused] = useState(false);
  const [activeSuggestionIndex, setActiveSuggestionIndex] = useState(0);
  const [prevInputValue, setPrevInputValue] = useState(inputValue);

  if (inputValue !== prevInputValue) {
    setPrevInputValue(inputValue);
    setActiveSuggestionIndex(0);
  }

  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Получаем контакты для автодополнения
  const { data: rawContacts } = useContacts(accountId);
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];

  // Обработка клика вне компонента для закрытия дропдауна
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsFocused(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Валидация email
  const isValidEmail = (email: string) => {
    const re = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    return re.test(email.trim());
  };

  // Добавление новой чипсы (email)
  const addEmails = (emailsToAdd: string[]) => {
    const validEmails = emailsToAdd
      .map((e) => e.trim())
      .filter((e) => e && !value.includes(e));

    if (validEmails.length > 0) {
      onChange([...value, ...validEmails]);
    }
    setInputValue("");
    setActiveSuggestionIndex(0);
  };

  // Удаление чипсы по индексу
  const removeEmail = (indexToRemove: number) => {
    onChange(value.filter((_, index) => index !== indexToRemove));
  };

  // Фильтрация контактов для подсказок
  const getSuggestions = () => {
    const trimmedInput = inputValue.trim().toLowerCase();

    // Если инпут пустой, показываем топ-5 контактов
    if (!trimmedInput) {
      return contacts.slice(0, 5);
    }

    // Иначе фильтруем по имени или email
    return contacts
      .filter(
        (c) =>
          c.name.toLowerCase().includes(trimmedInput) ||
          c.address.toLowerCase().includes(trimmedInput),
      )
      .slice(0, 8);
  };

  const suggestions = getSuggestions();

  // Обработка клавиш в инпуте
  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" || e.key === "," || e.key === ";") {
      e.preventDefault();
      if (suggestions.length > 0 && inputValue.trim()) {
        // Если есть выбранная подсказка, добавляем её
        const selectedContact = suggestions[activeSuggestionIndex];
        if (selectedContact) {
          addEmails([selectedContact.address]);
          return;
        }
      }

      // Иначе пытаемся добавить введенное значение как email
      if (inputValue.trim()) {
        if (isValidEmail(inputValue)) {
          addEmails([inputValue]);
        } else {
          // Выделяем инпут красным или просто не добавляем, если невалидный
          // Для премиального UX можно добавить легкую микро-анимацию
        }
      }
    } else if (e.key === "Backspace" && !inputValue && value.length > 0) {
      // Удаляем последний email при нажатии Backspace на пустом инпуте
      removeEmail(value.length - 1);
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveSuggestionIndex((prev) =>
        prev < suggestions.length - 1 ? prev + 1 : prev,
      );
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveSuggestionIndex((prev) => (prev > 0 ? prev - 1 : 0));
    } else if (e.key === "Escape") {
      setIsFocused(false);
    }
  };

  // Фокусировка на скрытом инпуте при клике на контейнер
  const handleContainerClick = () => {
    inputRef.current?.focus();
  };

  return (
    <div ref={containerRef} className="relative w-full">
      <div
        id={id}
        onClick={handleContainerClick}
        className={`flex flex-wrap gap-1.5 p-1.5 min-h-[40px] w-full rounded-md border bg-card-bg text-sm transition-all duration-200 cursor-text ${
          isFocused
            ? "border-primary ring-1 ring-primary/50 shadow-[0_0_10px_rgba(251,191,36,0.15)]"
            : "border-input hover:border-muted-foreground/30"
        }`}
      >
        {/* Список чипсов */}
        {value.map((email, idx) => {
          // Ищем контакт с таким email, чтобы красиво вывести имя вместо сырого адреса
          const contact =
            contacts.find(
              (c) => c.address.toLowerCase() === email.toLowerCase(),
            ) ?? null;
          const displayLabel = contact ? `${contact.name} (${email})` : email;

          return (
            <Badge
              key={email}
              variant="secondary"
              className="flex items-center gap-1 ps-2.5 pe-1 py-0.5 max-w-xs md:max-w-md animate-fade-in group hover:bg-secondary/80 transition-colors"
            >
              <span className="truncate">{displayLabel}</span>
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  removeEmail(idx);
                }}
                className="text-muted-foreground hover:text-foreground rounded-full p-0.5 group-hover:bg-background/50 transition-colors"
              >
                <X className="w-3 h-3" />
              </button>
            </Badge>
          );
        })}

        {/* Инпут ввода */}
        <input
          id={id ? `${id}-input` : undefined}
          ref={inputRef}
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          placeholder={
            value.length === 0
              ? placeholder || t("recipient_input_placeholder")
              : ""
          }
          className="flex-1 min-w-[120px] bg-transparent outline-none border-none py-0.5 text-foreground placeholder:text-muted-foreground"
        />
      </div>

      {/* Выпадающий список подсказок */}
      {isFocused && suggestions.length > 0 && (
        <div
          ref={dropdownRef}
          className="absolute z-50 left-0 right-0 mt-1 max-h-60 overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-lg animate-in fade-in slide-in-from-top-1 duration-150 backdrop-blur-md bg-opacity-95"
        >
          <div className="p-1">
            <div className="px-2 py-1 text-xs font-semibold text-muted-foreground border-b border-border-muted/50 mb-1">
              {!inputValue
                ? t("recipient_quick_select")
                : t("recipient_address_matches")}
            </div>
            {suggestions.map((c, index) => (
              <div
                key={c.address}
                onClick={(e) => {
                  e.stopPropagation();
                  addEmails([c.address]);
                }}
                onMouseEnter={() => setActiveSuggestionIndex(index)}
                className={`flex items-center justify-between px-3 py-2 text-sm rounded-sm cursor-pointer transition-colors ${
                  index === activeSuggestionIndex
                    ? "bg-accent text-accent-foreground"
                    : "hover:bg-muted"
                }`}
              >
                <div className="flex flex-col truncate">
                  <span className="font-medium truncate flex items-center gap-1.5">
                    <User className="w-3.5 h-3.5 opacity-60" />
                    {c.name}
                  </span>
                  <span className="text-xs text-muted-foreground truncate flex items-center gap-1.5 mt-0.5">
                    <Mail className="w-3 h-3 opacity-50" />
                    {c.address}
                  </span>
                </div>
                {index === activeSuggestionIndex && (
                  <span className="text-xs text-muted-foreground bg-background px-1.5 py-0.5 rounded border border-border-muted/50">
                    {t("key_enter")}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
