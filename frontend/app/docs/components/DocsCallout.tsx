import React from 'react';
import type { MDXComponents } from 'mdx/types';

export const DocsCallout: MDXComponents = {
  blockquote: ({ children, ...props }) => (
    <blockquote 
      className="mt-6 border-l-2 border-emerald-500 bg-emerald-50 dark:bg-emerald-950/20 pl-6 italic text-slate-800 dark:text-slate-200 py-4 pr-4 rounded-r-lg" 
      {...props}
    >
      {children}
    </blockquote>
  ),
};
