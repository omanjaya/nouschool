import { Fragment } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';

export interface BreadcrumbItemDef {
  label: string;
  /** Tautan — kosong untuk crumb terakhir (halaman sekarang, tidak bisa diklik). */
  to?: string;
}

/**
 * Breadcrumb top bar desktop (`AppShell`, docs/10 §5 "App bar halaman-dalam").
 * Pola dari `@shadcnblocks/sidebar1`, ditulis ulang jadi komponen berbasis
 * `items` (bukan compound `Breadcrumb.Item/.Link/.Separator` shadcn) — markup
 * breadcrumb murni struktural, tidak butuh primitif Radix.
 */
export function Breadcrumb({ items }: { items: BreadcrumbItemDef[] }) {
  if (items.length === 0) return null;
  return (
    <nav aria-label="Breadcrumb" className="flex min-w-0 items-center gap-1.5 text-[12px]">
      {items.map((item, i) => {
        const isLast = i === items.length - 1;
        return (
          <Fragment key={`${item.label}-${i}`}>
            {i > 0 && <ChevronRight size={14} strokeWidth={2} className="shrink-0 text-muted" aria-hidden="true" />}
            {item.to && !isLast ? (
              <Link to={item.to} className="truncate text-muted transition-colors duration-150 hover:text-ink">
                {item.label}
              </Link>
            ) : (
              <span className="truncate font-medium text-ink" aria-current={isLast ? 'page' : undefined}>
                {item.label}
              </span>
            )}
          </Fragment>
        );
      })}
    </nav>
  );
}
