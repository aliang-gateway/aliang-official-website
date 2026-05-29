import type { MDXComponents } from 'mdx/types';

export const DocsCallout: MDXComponents = {
  blockquote: ({ children, ...props }) => (
    <blockquote
      className="mt-6 border-l-2 border-[var(--stitch-primary)] bg-[var(--stitch-primary)]/5 pl-6 italic text-[var(--stitch-text-muted)] py-4 pr-4 rounded-r-lg"
      {...props}
    >
      {children}
    </blockquote>
  ),
};
