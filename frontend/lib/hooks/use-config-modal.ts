"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import type { ClientTemplateId, TemplateFormat } from "@/lib/dashboard-types";
import { TEMPLATE_DEFINITIONS, buildTemplateContent, type TemplateDefinition } from "@/lib/dashboard-template";

const DASHBOARD_CONFIG_KEY_STORAGE_KEY = "dashboard_config_user_key";

export type ConfigModalState = {
  isOpen: boolean;
  open: () => void;
  close: () => void;
  userKey: string;
  setUserKey: (value: string) => void;
  template: ClientTemplateId;
  setTemplate: (value: ClientTemplateId) => void;
  format: TemplateFormat;
  setFormat: (value: TemplateFormat) => void;
  templateDefinition: TemplateDefinition;
  renderedConfig: string;
  copyState: "idle" | "copied" | "error";
  handleCopy: () => Promise<void>;
};

export function useConfigModal(): ConfigModalState {
  const [hydrated, setHydrated] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [userKey, setUserKeyState] = useState("");
  const [template, setTemplateState] = useState<ClientTemplateId>("opencode");
  const [format, setFormatState] = useState<TemplateFormat>("json");
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");

  useEffect(() => {
    setHydrated(true);
    const storedUserKey = localStorage.getItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY) ?? "";
    setUserKeyState(storedUserKey);
  }, []);

  useEffect(() => {
    if (!hydrated) {
      return;
    }

    localStorage.setItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY, userKey);
  }, [hydrated, userKey]);

  const templateDefinition = useMemo(
    () => TEMPLATE_DEFINITIONS.find((item) => item.id === template) ?? TEMPLATE_DEFINITIONS[0],
    [template],
  );

  useEffect(() => {
    const nextFormat = templateDefinition.supportedFormats.includes(format)
      ? format
      : templateDefinition.supportedFormats[0];

    if (nextFormat !== format) {
      setFormatState(nextFormat);
    }
  }, [format, templateDefinition]);

  const renderedConfig = useMemo(() => {
    return buildTemplateContent(template, format, userKey.trim());
  }, [format, template, userKey]);

  const setUserKey = useCallback((value: string) => {
    setUserKeyState(value);
    setCopyState("idle");
  }, []);

  const setTemplate = useCallback((value: ClientTemplateId) => {
    setTemplateState(value);
    setCopyState("idle");
  }, []);

  const setFormat = useCallback((value: TemplateFormat) => {
    setFormatState(value);
    setCopyState("idle");
  }, []);

  const open = useCallback(() => {
    setIsOpen(true);
    setCopyState("idle");
  }, []);

  const close = useCallback(() => {
    setIsOpen(false);
    setCopyState("idle");
  }, []);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(renderedConfig);
      setCopyState("copied");
    } catch {
      setCopyState("error");
    }
  }, [renderedConfig]);

  return {
    isOpen,
    open,
    close,
    userKey,
    setUserKey,
    template,
    setTemplate,
    format,
    setFormat,
    templateDefinition,
    renderedConfig,
    copyState,
    handleCopy,
  };
}
