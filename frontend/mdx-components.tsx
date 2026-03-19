import type { MDXComponents } from "mdx/types";

export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    h2: ({ className = "", ...props }) => (
      <h2
        className={`text-2xl font-bold text-[var(--portal-ink)] mt-8 mb-3 ${className}`.trim()}
        {...props}
      />
    ),
    h3: ({ className = "", ...props }) => (
      <h3
        className={`text-xl font-semibold text-[var(--portal-ink)] mt-6 mb-2 ${className}`.trim()}
        {...props}
      />
    ),
    p: ({ className = "", ...props }) => (
      <p
        className={`text-[var(--portal-muted)] leading-7 ${className}`.trim()}
        {...props}
      />
    ),
    ul: ({ className = "", ...props }) => (
      <ul
        className={`list-disc pl-6 space-y-2 text-[var(--portal-muted)] ${className}`.trim()}
        {...props}
      />
    ),
    ol: ({ className = "", ...props }) => (
      <ol
        className={`list-decimal pl-6 space-y-2 text-[var(--portal-muted)] ${className}`.trim()}
        {...props}
      />
    ),
    li: ({ className = "", ...props }) => <li className={`${className}`.trim()} {...props} />,
    a: ({ className = "", ...props }) => (
      <a
        className={`text-emerald-600 dark:text-emerald-400 underline underline-offset-2 hover:text-emerald-500 dark:hover:text-emerald-300 transition-colors ${className}`.trim()}
        {...props}
      />
    ),
    code: ({ className = "", ...props }) => {
      if (className.includes("language-")) {
        return <code className={`${className}`.trim()} {...props} />;
      }

      return (
        <code
          className={`bg-black/10 dark:bg-black/40 px-1.5 py-0.5 rounded text-emerald-600 dark:text-emerald-400 ${className}`.trim()}
          {...props}
        />
      );
    },
    pre: ({ className = "", ...props }) => (
      <pre
        className={`bg-black/10 dark:bg-black/40 border border-[var(--portal-line)] rounded-xl p-4 overflow-x-auto text-sm text-emerald-600 dark:text-emerald-400 ${className}`.trim()}
        {...props}
      />
    ),
    strong: ({ className = "", ...props }) => (
      <strong className={`text-[var(--portal-ink)] font-semibold ${className}`.trim()} {...props} />
    ),
    ...components,
  };
}
