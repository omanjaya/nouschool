import { useMemo, useState, type ReactNode } from 'react';
import { ArrowDown, ArrowUp, ArrowUpDown, MoreHorizontal, type LucideIcon } from 'lucide-react';
import { Skeleton } from './Skeleton';
import { EmptyState } from './EmptyState';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './DropdownMenu';

export interface DataTableColumn<T> {
  key: string;
  header: string;
  cell: (row: T) => ReactNode;
  /** Kolom angka (docs/10 §6): rata kanan + `num` (tabular-nums). */
  align?: 'left' | 'right';
  sortable?: boolean;
  /** Wajib diisi kalau `sortable` — nilai mentah untuk dibandingkan (bukan `cell` yang bisa berupa JSX). */
  sortValue?: (row: T) => string | number;
  className?: string;
}

export interface DataTableAction<T> {
  label: string;
  icon?: LucideIcon;
  onClick: (row: T) => void;
  variant?: 'default' | 'danger';
}

interface DataTableProps<T> {
  columns: DataTableColumn<T>[];
  data: T[];
  keyField: (row: T) => string;
  isLoading?: boolean;
  skeletonRows?: number;
  emptyIcon?: LucideIcon;
  emptyMessage?: string;
  emptyAction?: ReactNode;
  actions?: (row: T) => DataTableAction<T>[];
  onRowClick?: (row: T) => void;
  className?: string;
}

/**
 * Tabel admin desktop (docs/10 §6): header `surface-2` 11px uppercase muted,
 * hairline antar-baris, kolom angka rata kanan tabular, header sticky, sort
 * klik-header (client-side), scroll-x, aksi baris via dropdown `MoreHorizontal`.
 * Dipakai sebagai varian desktop dari `ListRow` (docs/10 §5) — bukan pengganti,
 * mobile tetap `ListRow`.
 */
export function DataTable<T>({
  columns,
  data,
  keyField,
  isLoading = false,
  skeletonRows = 5,
  emptyIcon,
  emptyMessage = 'Belum ada data.',
  emptyAction,
  actions,
  onRowClick,
  className = '',
}: DataTableProps<T>) {
  const [sort, setSort] = useState<{ key: string; dir: 'asc' | 'desc' } | null>(null);

  const sortedData = useMemo(() => {
    if (!sort) return data;
    const column = columns.find((c) => c.key === sort.key);
    if (!column?.sortValue) return data;
    const copy = [...data];
    copy.sort((a, b) => {
      const va = column.sortValue!(a);
      const vb = column.sortValue!(b);
      const cmp = typeof va === 'number' && typeof vb === 'number' ? va - vb : String(va).localeCompare(String(vb), 'id');
      return sort.dir === 'asc' ? cmp : -cmp;
    });
    return copy;
  }, [data, sort, columns]);

  function toggleSort(key: string) {
    setSort((current) => {
      if (current?.key !== key) return { key, dir: 'asc' };
      if (current.dir === 'asc') return { key, dir: 'desc' };
      return null;
    });
  }

  const columnCount = columns.length + (actions ? 1 : 0);

  return (
    <div className={`overflow-x-auto rounded-lg border border-line ${className}`}>
      <table className="w-full min-w-max border-collapse text-[14px]">
        <thead className="sticky top-0 z-10 bg-surface-2">
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                scope="col"
                className={`border-b border-line px-3 py-2.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted ${
                  col.align === 'right' ? 'text-right' : 'text-left'
                } ${col.className ?? ''}`}
              >
                {col.sortable ? (
                  <button
                    type="button"
                    onClick={() => toggleSort(col.key)}
                    className={`inline-flex items-center gap-1 transition-colors duration-150 hover:text-ink ${
                      col.align === 'right' ? 'flex-row-reverse' : ''
                    }`}
                  >
                    {col.header}
                    {sort?.key === col.key ? (
                      sort.dir === 'asc' ? (
                        <ArrowUp size={14} strokeWidth={2} className="text-primary" aria-hidden="true" />
                      ) : (
                        <ArrowDown size={14} strokeWidth={2} className="text-primary" aria-hidden="true" />
                      )
                    ) : (
                      <ArrowUpDown size={14} strokeWidth={2} aria-hidden="true" />
                    )}
                  </button>
                ) : (
                  col.header
                )}
              </th>
            ))}
            {actions && (
              <th scope="col" className="border-b border-line px-3 py-2.5 w-10">
                <span className="sr-only">Aksi</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {isLoading ? (
            Array.from({ length: skeletonRows }).map((_, i) => (
              <tr key={i} className="border-b border-line last:border-0">
                {Array.from({ length: columnCount }).map((__, j) => (
                  <td key={j} className="px-3 py-3">
                    <Skeleton className="h-4 w-full" />
                  </td>
                ))}
              </tr>
            ))
          ) : sortedData.length === 0 ? (
            <tr>
              <td colSpan={columnCount} className="px-3 py-8">
                <EmptyState icon={emptyIcon} message={emptyMessage} action={emptyAction} />
              </td>
            </tr>
          ) : (
            sortedData.map((row) => (
              <tr
                key={keyField(row)}
                className={`border-b border-line last:border-0 ${
                  onRowClick ? 'cursor-pointer transition-colors duration-150 hover:bg-surface-2' : ''
                }`}
                onClick={onRowClick ? () => onRowClick(row) : undefined}
              >
                {columns.map((col) => (
                  <td
                    key={col.key}
                    className={`px-3 py-2.5 ${col.align === 'right' ? 'num text-right' : ''} ${col.className ?? ''}`}
                  >
                    {col.cell(row)}
                  </td>
                ))}
                {actions && (
                  <td className="px-3 py-2.5 text-right" onClick={(e) => e.stopPropagation()}>
                    <RowActionsMenu row={row} actions={actions(row)} />
                  </td>
                )}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}

function RowActionsMenu<T>({ row, actions }: { row: T; actions: DataTableAction<T>[] }) {
  if (actions.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Aksi baris"
          className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          <MoreHorizontal size={18} strokeWidth={2} aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {actions.map((action) => (
          <DropdownMenuItem key={action.label} variant={action.variant} onClick={() => action.onClick(row)}>
            {action.icon && <action.icon size={16} strokeWidth={2} aria-hidden="true" />}
            {action.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
