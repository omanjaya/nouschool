import type { ComponentProps } from 'react';
import { DropdownMenu as DropdownMenuPrimitive } from 'radix-ui';

/**
 * Dropdown menu — bahan dari `@shadcnblocks/dashboard1` (menu aksi baris
 * tabel, menu user), direstyle penuh ke token Rapor: radius 8, hairline
 * `border-line`, tanpa warna shadcn default (`popover`/`accent`/`destructive`
 * tidak ada di token kita → diganti `surface`/`surface-2`/`danger`).
 * Shadow overlay memakai token yang sama dengan `Dialog.tsx` (docs/10 §3).
 * Hanya varian yang dipakai (`DataTable` aksi baris, `AppShell` menu user) —
 * checkbox/radio/submenu shadcn dibuang, bukan dipertahankan "siapa tahu".
 */
function DropdownMenu({ ...props }: ComponentProps<typeof DropdownMenuPrimitive.Root>) {
  return <DropdownMenuPrimitive.Root data-slot="dropdown-menu" {...props} />;
}

function DropdownMenuTrigger({ ...props }: ComponentProps<typeof DropdownMenuPrimitive.Trigger>) {
  return <DropdownMenuPrimitive.Trigger data-slot="dropdown-menu-trigger" {...props} />;
}

function DropdownMenuContent({
  className = '',
  sideOffset = 4,
  ...props
}: ComponentProps<typeof DropdownMenuPrimitive.Content>) {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        data-slot="dropdown-menu-content"
        sideOffset={sideOffset}
        className={`z-50 min-w-[10rem] overflow-hidden rounded-lg border border-line bg-surface p-1 text-[14px] text-ink shadow-[0_8px_24px_rgba(23,35,58,0.12)] data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 ${className}`}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  );
}

function DropdownMenuItem({
  className = '',
  variant = 'default',
  ...props
}: ComponentProps<typeof DropdownMenuPrimitive.Item> & { variant?: 'default' | 'danger' }) {
  const variantClass =
    variant === 'danger'
      ? 'text-danger focus:bg-danger-soft focus:text-danger [&_svg]:text-danger'
      : 'text-ink focus:bg-surface-2 focus:text-ink [&_svg]:text-muted';
  return (
    <DropdownMenuPrimitive.Item
      data-slot="dropdown-menu-item"
      className={`relative flex min-h-9 cursor-default items-center gap-2 rounded-md px-2.5 py-1.5 text-[14px] outline-hidden select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 ${variantClass} ${className}`}
      {...props}
    />
  );
}

function DropdownMenuLabel({ className = '', ...props }: ComponentProps<typeof DropdownMenuPrimitive.Label>) {
  return (
    <DropdownMenuPrimitive.Label
      data-slot="dropdown-menu-label"
      className={`px-2.5 py-1.5 text-[12px] font-semibold text-muted ${className}`}
      {...props}
    />
  );
}

function DropdownMenuSeparator({ className = '', ...props }: ComponentProps<typeof DropdownMenuPrimitive.Separator>) {
  return (
    <DropdownMenuPrimitive.Separator
      data-slot="dropdown-menu-separator"
      className={`-mx-1 my-1 h-px bg-line ${className}`}
      {...props}
    />
  );
}

export { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator };
