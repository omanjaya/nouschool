import { NavLink } from 'react-router-dom';

export interface TabItem {
  to: string;
  label: string;
}

interface TabsProps {
  items: TabItem[];
}

/** Tab hairline (bukan SegmentedControl) — dipakai untuk sub-navigasi di dalam satu area, mis. /data. */
export function Tabs({ items }: TabsProps) {
  return (
    <nav aria-label="Tab" className="flex border-b border-line">
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className="relative flex min-h-11 items-center px-4 text-[14px] font-medium text-muted transition-colors duration-150 hover:text-ink aria-[current=page]:text-primary"
        >
          {({ isActive }) => (
            <>
              {item.label}
              {isActive && (
                <span aria-hidden="true" className="absolute inset-x-3 -bottom-px h-[2px] rounded-full bg-primary" />
              )}
            </>
          )}
        </NavLink>
      ))}
    </nav>
  );
}
