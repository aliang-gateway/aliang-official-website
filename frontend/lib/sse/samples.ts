export type SampleKey = "openai-text" | "openai-tool" | "anthropic-text" | "anthropic-tool";

/** 构造单个 SSE chunk：Anthropic 带 event 名，OpenAI 不带。 */
function chunk(eventName: string | undefined, data: unknown): string {
  const dataLine = data === "[DONE]" ? "[DONE]" : JSON.stringify(data);
  return eventName ? `event: ${eventName}\ndata: ${dataLine}` : `data: ${dataLine}`;
}
function stream(...chunks: string[]): string {
  return chunks.join("\n\n");
}

export const SAMPLES: Record<SampleKey, { labelZh: string; labelEn: string; raw: string }> = {
  "openai-text": {
    labelZh: "OpenAI 文本流",
    labelEn: "OpenAI text stream",
    raw: stream(
      chunk(undefined, { id: "chatcmpl-x", object: "chat.completion.chunk", model: "gpt-4o", choices: [{ index: 0, delta: { role: "assistant", content: "" }, finish_reason: null }] }),
      chunk(undefined, { id: "chatcmpl-x", object: "chat.completion.chunk", choices: [{ index: 0, delta: { content: "Hello" }, finish_reason: null }] }),
      chunk(undefined, { id: "chatcmpl-x", object: "chat.completion.chunk", choices: [{ index: 0, delta: { content: " world" }, finish_reason: null }] }),
      chunk(undefined, { id: "chatcmpl-x", object: "chat.completion.chunk", choices: [{ index: 0, delta: {}, finish_reason: "stop" }], usage: { prompt_tokens: 9, completion_tokens: 2, total_tokens: 11 } }),
      chunk(undefined, "[DONE]"),
    ),
  },
  "openai-tool": {
    labelZh: "OpenAI 工具调用",
    labelEn: "OpenAI tool calls",
    raw: stream(
      chunk(undefined, { model: "gpt-4o", choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: "call_1", type: "function", function: { name: "get_weather", arguments: "" } }] } }] }),
      chunk(undefined, { choices: [{ index: 0, delta: { tool_calls: [{ index: 0, function: { arguments: "{\"city\":" } }] } }] }),
      chunk(undefined, { choices: [{ index: 0, delta: { tool_calls: [{ index: 0, function: { arguments: "\"Beijing\"}" } }] } }] }),
      chunk(undefined, { choices: [{ index: 0, delta: {}, finish_reason: "tool_calls" }] }),
      chunk(undefined, "[DONE]"),
    ),
  },
  "anthropic-text": {
    labelZh: "Anthropic 文本流",
    labelEn: "Anthropic text stream",
    raw: stream(
      chunk("message_start", { type: "message_start", message: { id: "msg_x", model: "claude-sonnet", role: "assistant", content: [], stop_reason: null, usage: { input_tokens: 10, output_tokens: 1, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 } } }),
      chunk("content_block_start", { type: "content_block_start", index: 0, content_block: { type: "text", text: "" } }),
      chunk("content_block_delta", { type: "content_block_delta", index: 0, delta: { type: "text_delta", text: "Hello" } }),
      chunk("content_block_delta", { type: "content_block_delta", index: 0, delta: { type: "text_delta", text: "!" } }),
      chunk("content_block_stop", { type: "content_block_stop", index: 0 }),
      chunk("message_delta", { type: "message_delta", delta: { stop_reason: "end_turn", stop_sequence: null }, usage: { output_tokens: 4 } }),
      chunk("message_stop", { type: "message_stop" }),
    ),
  },
  "anthropic-tool": {
    labelZh: "Anthropic 工具调用",
    labelEn: "Anthropic tool use",
    raw: stream(
      chunk("content_block_start", { type: "content_block_start", index: 0, content_block: { type: "tool_use", id: "toolu_1", name: "get_weather", input: {} } }),
      chunk("content_block_delta", { type: "content_block_delta", index: 0, delta: { type: "input_json_delta", partial_json: "{\"city\":" } }),
      chunk("content_block_delta", { type: "content_block_delta", index: 0, delta: { type: "input_json_delta", partial_json: "\"Beijing\"}" } }),
      chunk("content_block_stop", { type: "content_block_stop", index: 0 }),
      chunk("message_delta", { type: "message_delta", delta: { stop_reason: "tool_use" }, usage: { output_tokens: 20 } }),
      chunk("message_stop", { type: "message_stop" }),
    ),
  },
};
