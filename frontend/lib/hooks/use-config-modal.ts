"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import type { ClientTemplateId, TemplateFormat } from "@/lib/dashboard-types";
import { TEMPLATE_DEFINITIONS, buildTemplateContent, type TemplateDefinition } from "@/lib/dashboard-template";

const DASHBOARD_CONFIG_KEY_STORAGE_KEY = "dashboard_config_user_key";
/** 拉取可用模型列表的去抖时长(ms),避免输入过程中频繁请求。 */
const MODELS_FETCH_DEBOUNCE_MS = 400;

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
  // 当前密钥实际可用的模型列表(经网关 /v1/models)。空表示未拉到/未提供密钥,
  // 此时生成的配置会回退到各模板的内置默认模型。
  const [availableModels, setAvailableModels] = useState<string[]>([]);

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

  // 拉取所选密钥实际可用的模型列表,供生成的配置填入真实 model。
  // 带去抖与取消:输入过程中只保留最后一次请求;所有 setState 都在异步回调里,
  // 不在 effect 同步执行,避免触发 react-hooks/set-state-in-effect。
  useEffect(() => {
    const trimmed = userKey.trim();
    let cancelled = false;
    const timer = setTimeout(async () => {
      if (!trimmed) {
        if (!cancelled) setAvailableModels([]);
        return;
      }
      try {
        const res = await fetch("/api/models", {
          headers: { "x-api-key": trimmed },
          cache: "no-store",
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const payload = (await res.json()) as { data?: unknown };
        const list = Array.isArray(payload?.data) ? payload.data : [];
        const names = list
          .map((item: unknown) =>
            typeof item === "string"
              ? item
              : (item as { id?: string; name?: string })?.id ?? (item as { name?: string })?.name,
          )
          .filter((name): name is string => Boolean(name));
        if (!cancelled) setAvailableModels(names);
      } catch {
        if (!cancelled) setAvailableModels([]);
      }
    }, MODELS_FETCH_DEBOUNCE_MS);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [userKey]);

  const renderedConfig = useMemo(() => {
    return buildTemplateContent(template, userKey.trim(), availableModels);
  }, [template, userKey, availableModels]);

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
