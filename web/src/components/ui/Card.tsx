import type { HTMLAttributes, ReactNode } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'plain';
  children: ReactNode;
}

/** Permukaan ber-hairline radius 8. Varian `plain` = tanpa border, untuk grouping. */
export function Card({ variant = 'default', className = '', children, ...rest }: CardProps) {
  const border = variant === 'plain' ? '' : 'border border-line';
  return (
    <div className={`rounded-lg bg-surface p-4 ${border} ${className}`} {...rest}>
      {children}
    </div>
  );
}
