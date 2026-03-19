import type { MDXComponents } from 'mdx/types';
import { DocsMDXComponents } from './app/docs/components/DocsMDXComponents';

export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    ...components,
    ...DocsMDXComponents,
  };
}
