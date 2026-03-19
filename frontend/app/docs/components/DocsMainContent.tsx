import React from 'react';
import type { MDXComponents } from 'mdx/types';

export const DocsMainContent: MDXComponents = {
  h1: ({ children, ...props }) => (
    <h1 className="text-4xl font-extrabold tracking-tight mb-8 text-slate-900 dark:text-slate-100" {...props}>
      {children}
    </h1>
  ),
  h2: ({ children, ...props }) => (
    <h2 className="text-3xl font-bold tracking-tight mt-12 mb-6 text-slate-900 dark:text-slate-100 pb-2 border-b border-slate-200 dark:border-slate-800" {...props}>
      {children}
    </h2>
  ),
  h3: ({ children, ...props }) => (
    <h3 className="text-2xl font-semibold tracking-tight mt-8 mb-4 text-slate-900 dark:text-slate-100" {...props}>
      {children}
    </h3>
  ),
  h4: ({ children, ...props }) => (
    <h4 className="text-xl font-semibold tracking-tight mt-6 mb-4 text-slate-900 dark:text-slate-100" {...props}>
      {children}
    </h4>
  ),
  p: ({ children, ...props }) => (
    <p className="leading-7 text-slate-700 dark:text-slate-300 mb-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ children, ...props }) => (
    <ul className="my-6 ml-6 list-disc [&>li]:mt-2 text-slate-700 dark:text-slate-300 marker:text-slate-400" {...props}>
      {children}
    </ul>
  ),
  ol: ({ children, ...props }) => (
    <ol className="my-6 ml-6 list-decimal [&>li]:mt-2 text-slate-700 dark:text-slate-300 marker:text-slate-400" {...props}>
      {children}
    </ol>
  ),
  li: ({ children, ...props }) => (
    <li className="leading-7" {...props}>
      {children}
    </li>
  ),
  a: ({ children, href, ...props }) => {
    const isInternal = href && href.startsWith('/');
    return (
      <a
        href={href}
        className="font-medium text-emerald-600 dark:text-emerald-400 underline underline-offset-4 hover:text-emerald-700 dark:hover:text-emerald-300 transition-colors"
        {...(!isInternal ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        {...props}
      >
        {children}
      </a>
    );
  },
  strong: ({ children, ...props }) => (
    <strong className="font-semibold text-slate-900 dark:text-slate-100" {...props}>
      {children}
    </strong>
  ),
  hr: (props) => (
    <hr className="my-8 border-slate-200 dark:border-slate-800" {...props} />
  ),
};
