import type { ComponentProps } from 'react';
import { Tooltip as TooltipPrimitive } from 'radix-ui';

/**
 * Tooltip kecil — dipakai `AppShell` untuk label item nav saat sidebar
 * desktop collapsed (ikon saja). Bahan dari `@shadcnblocks/dashboard1`,
 * direstyle: latar `--ink` (bukan `--foreground` shadcn), radius 8 (bukan
 * `rounded-md` default), shadow overlay token `Dialog.tsx` (docs/10 §3).
 */
function TooltipProvider({ delayDuration = 200, ...props }: ComponentProps<typeof TooltipPrimitive.Provider>) {
  return <TooltipPrimitive.Provider data-slot="tooltip-provider" delayDuration={delayDuration} {...props} />;
}

function Tooltip({ ...props }: ComponentProps<typeof TooltipPrimitive.Root>) {
  return <TooltipPrimitive.Root data-slot="tooltip" {...props} />;
}

function TooltipTrigger({ ...props }: ComponentProps<typeof TooltipPrimitive.Trigger>) {
  return <TooltipPrimitive.Trigger data-slot="tooltip-trigger" {...props} />;
}

function TooltipContent({
  className = '',
  sideOffset = 6,
  children,
  ...props
}: ComponentProps<typeof TooltipPrimitive.Content>) {
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        data-slot="tooltip-content"
        sideOffset={sideOffset}
        className={`z-50 rounded-lg bg-ink px-2.5 py-1.5 text-[12px] text-surface shadow-[0_8px_24px_rgba(23,35,58,0.12)] data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 ${className}`}
        {...props}
      >
        {children}
      </TooltipPrimitive.Content>
    </TooltipPrimitive.Portal>
  );
}

export { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider };
