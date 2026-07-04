import type { MDXComponents } from 'mdx/types';

/**
 * Pass-through MDX element overrides. Typography is owned by `.editorial
 * .prose` (editorial.css); we only keep behaviour that CSS cannot express:
 * external links open in a new tab, and tables get a horizontal scroll wrapper.
 */
export const DocsMainContent: MDXComponents = {
  a: ({ children, href, ...props }) => {
    const isInternal = href && href.startsWith('/');
    return (
      <a
        href={href}
        {...(!isInternal ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
        {...props}
      >
        {children}
      </a>
    );
  },
  img: ({ ...props }) => <img {...props} />,
  table: ({ children, ...props }) => (
    <div className="docs-table-wrap">
      <table {...props}>{children}</table>
    </div>
  ),
};
