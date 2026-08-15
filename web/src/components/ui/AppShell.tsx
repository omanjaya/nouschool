import type { ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import { LogOut } from 'lucide-react';
import type { NavItemDef } from '../../lib/nav';

interface AppShellProps {
  children: ReactNode;
  navItems: NavItemDef[];
  userName?: string;
  onLogout?: () => void;
}

/**
 * Bottom tab bar di mobile (<1024px), sidebar kiri di desktop (>=1024px).
 * Item aktif: warna --primary; underline 2px hanya di mobile.
 */
export function AppShell({ children, navItems, userName, onLogout }: AppShellProps) {
  return (
    <div className="min-h-dvh bg-bg text-ink lg:flex">
      <aside className="hidden lg:flex lg:h-dvh lg:w-60 lg:shrink-0 lg:flex-col lg:border-r lg:border-line lg:py-6 print:hidden">
        <div className="px-5 pb-6">
          <span className="text-[18px] font-semibold text-ink">NouSchool</span>
        </div>
        <nav className="flex flex-col gap-1 px-3">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                `flex min-h-11 items-center gap-3 rounded-lg px-3 text-[14px] font-medium transition-colors duration-150 ${
                  isActive ? 'bg-primary-soft text-primary' : 'text-muted hover:bg-surface-2 hover:text-ink'
                }`
              }
            >
              <item.icon size={20} strokeWidth={2} aria-hidden="true" />
              {item.label}
            </NavLink>
          ))}
        </nav>

        {userName && (
          <div className="mt-auto flex items-center justify-between gap-2 border-t border-line px-5 pt-4">
            <span className="truncate text-[14px] font-medium text-ink">{userName}</span>
            {onLogout && (
              <button
                type="button"
                onClick={onLogout}
                aria-label="Keluar"
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
              >
                <LogOut size={20} strokeWidth={2} aria-hidden="true" />
              </button>
            )}
          </div>
        )}
      </aside>

      <div className="flex min-h-dvh flex-1 flex-col lg:min-h-0">
        <main className="flex-1 pb-[68px] lg:pb-0 print:pb-0">{children}</main>

        <nav
          aria-label="Navigasi utama"
          className="fixed inset-x-0 bottom-0 z-10 border-t border-line bg-surface lg:hidden print:hidden"
        >
          <ul className="flex">
            {navItems.map((item) => (
              <li key={item.to} className="flex-1">
                <NavLink
                  to={item.to}
                  end={item.to === '/'}
                  className="relative flex min-h-[56px] flex-col items-center justify-center gap-1 py-2"
                >
                  {({ isActive }) => (
                    <>
                      {isActive && (
                        <span
                          aria-hidden="true"
                          className="absolute top-0 h-[2px] w-8 rounded-full bg-primary"
                        />
                      )}
                      <item.icon
                        size={20}
                        strokeWidth={2}
                        aria-hidden="true"
                        className={isActive ? 'text-primary' : 'text-muted'}
                      />
                      <span className={`text-[11px] ${isActive ? 'text-primary' : 'text-muted'}`}>
                        {item.label}
                      </span>
                    </>
                  )}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </div>
  );
}
