import type { ToolCall } from "./types";

/** 把按 index 累加好的 arguments 字符串整体 JSON.parse；失败保留字符串并记 error。 */
export function finalizeToolCalls(map: Map<number, ToolCall>): ToolCall[] {
  return Array.from(map.values())
    .sort((a, b) => a.index - b.index)
    .map((tc) => {
      const argsStr = typeof tc.arguments === "string" ? tc.arguments : "";
      if (argsStr === "") return { ...tc, arguments: {} };
      try {
        return { ...tc, arguments: JSON.parse(argsStr) as object };
      } catch (e) {
        return {
          ...tc,
          arguments: argsStr,
          argumentsParseError: e instanceof Error ? e.message : "parse error",
        };
      }
    });
}
