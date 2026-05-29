import type { MDXComponents } from 'mdx/types';

export const DocsCodeBlocks: MDXComponents = {
  pre: ({ children, ...props }) => (
    <pre
      className="mb-6 mt-6 overflow-x-auto rounded-xl bg-[var(--stitch-bg-elevated)] dark:bg-black py-4 px-4 border border-[var(--stitch-border)]"
      {...props}
    >
      {children}
    </pre>
  ),
  code: ({ className, children, ...props }) => {
    const isInlineCode = !className;

    if (isInlineCode) {
      return (
        <code
          className="relative rounded bg-[var(--stitch-bg-elevated)] px-[0.3rem] py-[0.2rem] font-mono text-sm font-medium text-[var(--stitch-primary)]"
          {...props}
        >
          {children}
        </code>
      );
    }

    return (
      <code
        className={`relative font-mono text-sm text-[var(--stitch-text)] ${className || ''}`}
        {...props}
      >
        {children}
      </code>
    );
  },
};
