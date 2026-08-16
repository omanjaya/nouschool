import { NavLink } from 'react-router-dom';

export interface TabItem {
  to: string;
  label: string;
  /**
   * Cocokkan hanya path persis (react-router `NavLink` prop `end`) — WAJIB
   * `true` untuk tab yang path-nya prefix dari tab lain (mis. `/kedisiplinan`
   * jadi prefix `/kedisiplinan/rekap`), supaya tidak ikut aktif di sub-rute
   * sibling-nya. Default `false` (perilaku lama, cocok untuk tab yang
   * segmen-nya sudah unik seperti `/data/siswa` vs `/data/rombel`).
   */
  end?: boolean;
}

interface TabsProps {
  items: TabItem[];
}

/** Tab hairline (bukan SegmentedControl) — dipakai untuk sub-navigasi di dalam satu area, mis. /data. */
export function Tabs({ items }: TabsProps) {
  return (
    <nav aria-label="Tab" className="overflow-x-auto border-b border-line">
      <div className="flex w-max min-w-full">
        {items.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className="relative flex min-h-11 shrink-0 items-center whitespace-nowrap px-4 text-[14px] font-medium text-muted transition-colors duration-150 hover:text-ink aria-[current=page]:text-primary"
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
      </div>
    </nav>
  );
}
