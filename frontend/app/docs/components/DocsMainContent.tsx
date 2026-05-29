import type { MDXComponents } from 'mdx/types';

export const DocsMainContent: MDXComponents = {
  h1: ({ children, ...props }) => (
    <h1 className="text-4xl font-extrabold tracking-tight mb-8 text-[var(--stitch-text)]" {...props}>
      {children}
    </h1>
  ),
  h2: ({ children, ...props }) => (
    <h2 className="text-3xl font-bold tracking-tight mt-12 mb-6 text-[var(--stitch-text)] pb-2 border-b border-[var(--stitch-border)]" {...props}>
      {children}
    </h2>
  ),
  h3: ({ children, ...props }) => (
    <h3 className="text-2xl font-semibold tracking-tight mt-8 mb-4 text-[var(--stitch-text)]" {...props}>
      {children}
    </h3>
  ),
  h4: ({ children, ...props }) => (
    <h4 className="text-xl font-semibold tracking-tight mt-6 mb-4 text-[var(--stitch-text)]" {...props}>
      {children}
    </h4>
  ),
  p: ({ children, ...props }) => (
    <p className="leading-7 text-[var(--stitch-text-muted)] mb-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ children, ...props }) => (
    <ul className="my-6 ml-6 list-disc [&>li]:mt-2 text-[var(--stitch-text-muted)] marker:text-[var(--stitch-text-muted)]/50" {...props}>
      {children}
    </ul>
  ),
  ol: ({ children, ...props }) => (
    <ol className="my-6 ml-6 list-decimal [&>li]:mt-2 text-[var(--stitch-text-muted)] marker:text-[var(--stitch-text-muted)]/50" {...props}>
      {children}
    </ol>
  ),
  li: ({ children, ...props }) => (
    <li className="leading-7 text-[var(--stitch-text-muted)]" {...props}>
      {children}
    </li>
  ),
  a: ({ children, href, ...props }) => {
    const isInternal = href && href.startsWith('/');
    return (
      <a
        href={href}
        className="font-medium text-[var(--stitch-primary)] underline underline-offset-4 hover:opacity-80 transition-colors"
        {...(!isInternal ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        {...props}
      >
        {children}
      </a>
    );
  },
  strong: ({ children, ...props }) => (
    <strong className="font-semibold text-[var(--stitch-text)]" {...props}>
      {children}
    </strong>
  ),
  em: ({ children, ...props }) => (
    <em className="italic" {...props}>
      {children}
    </em>
  ),
  del: ({ children, ...props }) => (
    <del className="line-through opacity-60" {...props}>
      {children}
    </del>
  ),
  hr: (props) => (
    <hr className="my-8 border-[var(--stitch-border)]" {...props} />
  ),
  img: ({ ...props }) => (
    <img className="max-w-full h-auto rounded-lg mx-auto my-6" {...props} />
  ),
  table: ({ children, ...props }) => (
    <div className="my-6 overflow-x-auto rounded-lg border border-[var(--stitch-border)]">
      <table className="w-full border-collapse text-sm" {...props}>
        {children}
      </table>
    </div>
  ),
  thead: ({ children, ...props }) => (
    <thead className="bg-[var(--stitch-bg-elevated)]" {...props}>
      {children}
    </thead>
  ),
  tbody: ({ children, ...props }) => (
    <tbody {...props}>
      {children}
    </tbody>
  ),
  tr: ({ children, ...props }) => (
    <tr className="border-b border-[var(--stitch-border)] even:bg-[var(--stitch-bg-elevated)]/50" {...props}>
      {children}
    </tr>
  ),
  th: ({ children, ...props }) => (
    <th className="px-4 py-3 text-left font-semibold text-[var(--stitch-text)] whitespace-nowrap" {...props}>
      {children}
    </th>
  ),
  td: ({ children, ...props }) => (
    <td className="px-4 py-3 text-[var(--stitch-text-muted)]" {...props}>
      {children}
    </td>
  ),
};
