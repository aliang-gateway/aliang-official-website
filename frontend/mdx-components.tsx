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
        className={`bg-black/10 dark:bg-black/40 border border-[var(--portal-line)] rounded-xl p-4 overflow-x-auto text-sm ${className}`.trim()}
        {...props}
      />
    ),
    table: ({ className = "", ...props }) => (
      <div className="my-6 overflow-x-auto rounded-xl border border-[var(--portal-line)]">
        <table className={`min-w-full border-collapse text-left text-sm ${className}`.trim()} {...props} />
      </div>
    ),
    thead: ({ className = "", ...props }) => (
      <thead className={`bg-black/5 dark:bg-white/5 ${className}`.trim()} {...props} />
    ),
    tbody: ({ className = "", ...props }) => <tbody className={`${className}`.trim()} {...props} />,
    tr: ({ className = "", ...props }) => (
      <tr
        className={`border-b border-[var(--portal-line)] odd:bg-transparent dark:odd:bg-transparent even:bg-black/5 dark:even:bg-white/5 ${className}`.trim()}
        {...props}
      />
    ),
    th: ({ className = "", ...props }) => (
      <th
        className={`px-4 py-3 font-semibold text-[var(--portal-ink)] whitespace-nowrap ${className}`.trim()}
        {...props}
      />
    ),
    td: ({ className = "", ...props }) => (
      <td className={`px-4 py-3 text-[var(--portal-muted)] align-top ${className}`.trim()} {...props} />
    ),
    blockquote: ({ className = "", ...props }) => (
      <blockquote
        className={`my-4 border-l-4 border-emerald-500/70 dark:border-emerald-400/70 pl-4 py-1 text-[var(--portal-muted)] italic bg-black/5 dark:bg-white/5 rounded-r-lg ${className}`.trim()}
        {...props}
      />
    ),
    hr: ({ className = "", ...props }) => (
      <hr className={`my-8 border-0 border-t border-[var(--portal-line)] ${className}`.trim()} {...props} />
    ),
    del: ({ className = "", ...props }) => (
      <del className={`text-[var(--portal-muted)] line-through ${className}`.trim()} {...props} />
    ),
    em: ({ className = "", ...props }) => (
      <em className={`italic text-[var(--portal-ink)] ${className}`.trim()} {...props} />
    ),
    strong: ({ className = "", ...props }) => (
      <strong className={`text-[var(--portal-ink)] font-semibold ${className}`.trim()} {...props} />
    ),
    img: ({ className = "", alt = "", ...props }) => (
      <img
        className={`max-w-full h-auto rounded-lg ${className}`.trim()}
        alt={alt}
        {...props}
      />
    ),
    ...components,
  };
}
