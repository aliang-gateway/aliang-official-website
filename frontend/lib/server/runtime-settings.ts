import "server-only";

import fs from "node:fs";
import path from "node:path";

type RuntimeSettingsFile = {
  api_base_url?: string;
  apiBaseUrl?: string;
  stripe_currency?: string;
  stripeCurrency?: string;
};

type RuntimeSettings = {
  apiBaseUrl?: string;
  stripeCurrency?: string;
};

const CONFIG_PATH_ENV_KEYS = ["FRONTEND_CONFIG_PATH", "CONFIG_PATH"] as const;
const DEFAULT_CONFIG_FILENAMES = ["config.json", "config.yaml", "config.yml"] as const;

let cachedSettings: RuntimeSettings | undefined;

function resolveConfigPath(): string | null {
  for (const key of CONFIG_PATH_ENV_KEYS) {
    const configured = process.env[key]?.trim();
    if (configured) {
      const resolved = path.isAbsolute(configured)
        ? configured
        : path.join(process.cwd(), configured);
      return fs.existsSync(resolved) ? resolved : null;
    }
  }

  for (const filename of DEFAULT_CONFIG_FILENAMES) {
    const resolved = path.join(process.cwd(), filename);
    if (fs.existsSync(resolved)) {
      return resolved;
    }
  }

  return null;
}

function parseYamlScalar(content: string, key: string): string | undefined {
  const pattern = new RegExp(`^${key}:\\s*"?([^"\\n#]+)"?\\s*$`, "m");
  const match = content.match(pattern);
  return match?.[1]?.trim();
}

function loadSettingsFile(configPath: string): RuntimeSettings {
  const raw = fs.readFileSync(configPath, "utf8");
  const ext = path.extname(configPath).toLowerCase();

  if (ext === ".json") {
    const parsed = JSON.parse(raw) as RuntimeSettingsFile;
    return {
      apiBaseUrl: parsed.api_base_url?.trim() || parsed.apiBaseUrl?.trim(),
      stripeCurrency: parsed.stripe_currency?.trim() || parsed.stripeCurrency?.trim(),
    };
  }

  return {
    apiBaseUrl: parseYamlScalar(raw, "api_base_url"),
    stripeCurrency: parseYamlScalar(raw, "stripe_currency"),
  };
}

function loadSettingsFromFile(): RuntimeSettings {
  const configPath = resolveConfigPath();
  if (!configPath) {
    return {};
  }

  try {
    return loadSettingsFile(configPath);
  } catch {
    return {};
  }
}

export function getRuntimeSettings(): RuntimeSettings {
  if (cachedSettings !== undefined) {
    return cachedSettings;
  }

  cachedSettings = loadSettingsFromFile();
  return cachedSettings;
}

export function readNonEmptyEnv(keys: readonly string[]): string | undefined {
  for (const key of keys) {
    const value = process.env[key]?.trim();
    if (value) {
      return value;
    }
  }
  return undefined;
}
