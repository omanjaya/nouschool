import { useEffect, useState, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { Bell, LogOut, PanelLeft, User } from 'lucide-react';
import type { NavGroup, NavItemDef } from '../../lib/nav';
import { getBreadcrumbItems } from '../../lib/breadcrumb';
import { Breadcrumb } from './Breadcrumb';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './Tooltip';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger, DropdownMenuSeparator } from './DropdownMenu';

interface AppShellProps {
  children: ReactNode;
  navItems: NavItemDef[];
  /** Grup nav lengkap untuk sidebar desktop (Fase 16, `lib/nav.ts#getSidebarNav`) — mobile tetap pakai `navItems` (≤5 tab). Kalau tidak diisi, sidebar jatuh ke `navItems` tanpa grup. */
  sidebarGroups?: NavGroup[];
  userName?: string;
  /** Label peran di bawah nama (blok user sidebar desktop), mis. "Admin Sekolah" — lihat `lib/roles.ts`. */
  roleLabel?: string;
  onLogout?: () => void;
  /** Angka badge per path nav (mis. unread notifikasi) — 0/undefined = tidak tampil. */
  badgeCounts?: Record<string, number>;
  /** Nama aplikasi di sidebar desktop — default "NouSchool" (docs/01 §branding). */
  appName?: string;
  /** Logo kecil di sebelah nama, ditampilkan hanya kalau ada (docs/01 §branding). */
  logoUrl?: string | null;
}

const SIDEBAR_COLLAPSED_KEY = 'nouschool:sidebar-collapsed';

function readStoredCollapsed(): boolean {
  if (typeof window === 'undefined') return false;
  return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1';
}

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
}

function NavBadge({ count }: { count: number }) {
  if (count <= 0) return null;
  return (
    <span
      aria-hidden="true"
      className="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-ink"
    >
      {count > 99 ? '99+' : count}
    </span>
  );
}

/** Badge titik kecil di atas ikon — dipakai nav sidebar saat collapsed (ikon saja, tanpa ruang untuk pill angka). */
function NavBadgeDot({ count }: { count: number }) {
  if (count <= 0) return null;
  return (
    <span
      aria-hidden="true"
      className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[9px] font-semibold leading-none text-primary-ink"
    >
      {count > 99 ? '99+' : count}
    </span>
  );
}

/**
 * Top bar tipis konten desktop (docs/10 §5 "App bar halaman-dalam"): breadcrumb
 * dari route + slot aksi kanan (notifikasi, menu user). Pola dari
 * `@shadcnblocks/dashboard1` (header) & `sidebar1` (breadcrumb), direstyle ke
 * token Rapor — tanpa search bar/kbd shortcut bawaan block (di luar scope).
 */
function DesktopTopBar({
  navItems,
  badgeCounts,
  userName,
  roleLabel,
  onLogout,
}: {
  navItems: NavItemDef[];
  badgeCounts?: Record<string, number>;
  userName?: string;
  roleLabel?: string;
  onLogout?: () => void;
}) {
  const location = useLocation();
  const navigate = useNavigate();
  const breadcrumbItems = getBreadcrumbItems(location.pathname);
  const notifItem = navItems.find((item) => item.icon === Bell);
  const notifCount = notifItem ? (badgeCounts?.[notifItem.to] ?? 0) : 0;

  return (
    <div className="hidden h-14 shrink-0 items-center justify-between border-b border-line px-6 lg:flex print:hidden">
      <Breadcrumb items={breadcrumbItems} />
      <div className="flex items-center gap-1.5">
        {notifItem && (
          <button
            type="button"
            onClick={() => navigate(notifItem.to)}
            aria-label={`Notifikasi${notifCount > 0 ? `, ${notifCount} belum dibaca` : ''}`}
            className="relative flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
          >
            <Bell size={20} strokeWidth={2} aria-hidden="true" />
            <NavBadgeDot count={notifCount} />
          </button>
        )}
        {userName && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={`Menu akun ${userName}`}
                className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-soft text-[13px] font-semibold text-primary transition-opacity duration-150 hover:opacity-80"
              >
                {getInitials(userName)}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <div className="px-2.5 py-1.5">
                <p className="truncate text-[14px] font-medium text-ink">{userName}</p>
                {roleLabel && <p className="truncate text-[12px] text-muted">{roleLabel}</p>}
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => navigate('/profil')}>
                <User size={16} strokeWidth={2} aria-hidden="true" />
                Profil
              </DropdownMenuItem>
              {onLogout && (
                <DropdownMenuItem variant="danger" onClick={onLogout}>
                  <LogOut size={16} strokeWidth={2} aria-hidden="true" />
                  Keluar
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </div>
  );
}

/**
 * Bottom tab bar di mobile (<1024px), sidebar kiri di desktop (>=1024px).
 * Item aktif: warna --primary; underline 2px hanya di mobile. Desktop sidebar
 * bisa diciutkan (ikon saja + tooltip nama, state disimpan di localStorage) —
 * pola dari `@shadcnblocks/sidebar1`. Mobile tidak berubah.
 */
export function AppShell({
  children,
  navItems,
  sidebarGroups,
  userName,
  roleLabel,
  onLogout,
  badgeCounts,
  appName = 'NouSchool',
  logoUrl = null,
}: AppShellProps) {
  const [collapsed, setCollapsed] = useState(readStoredCollapsed);
  const groups: NavGroup[] = sidebarGroups ?? [{ items: navItems }];

  useEffect(() => {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0');
  }, [collapsed]);

  return (
    <TooltipProvider>
      <div className="min-h-dvh bg-bg text-ink lg:flex lg:h-dvh lg:overflow-hidden">
        <aside
          className={`hidden lg:sticky lg:top-0 lg:flex lg:h-dvh lg:shrink-0 lg:flex-col lg:overflow-y-auto lg:border-r lg:border-line lg:py-6 print:hidden ${
            collapsed ? 'lg:w-[68px]' : 'lg:w-60'
          }`}
        >
          <div className={`flex items-center gap-2 pb-6 ${collapsed ? 'flex-col px-3' : 'px-5'}`}>
            <div className={`flex items-center gap-2 ${collapsed ? '' : 'min-w-0 flex-1'}`}>
              {logoUrl ? (
                <img src={logoUrl} alt="" className="h-8 w-8 shrink-0 rounded-lg object-contain" />
              ) : (
                <span
                  aria-hidden="true"
                  className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary-soft text-[14px] font-semibold text-primary"
                >
                  {appName.charAt(0).toUpperCase()}
                </span>
              )}
              {!collapsed && <span className="truncate text-[18px] font-semibold text-ink">{appName}</span>}
            </div>
            <button
              type="button"
              onClick={() => setCollapsed((c) => !c)}
              aria-label={collapsed ? 'Perluas sidebar' : 'Ciutkan sidebar'}
              aria-pressed={collapsed}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
            >
              <PanelLeft size={18} strokeWidth={2} aria-hidden="true" />
            </button>
          </div>
          <nav className={`flex flex-col gap-4 ${collapsed ? 'items-center px-2' : 'px-3'}`}>
            {groups.map((group, groupIndex) => (
              <div
                key={group.title ?? `group-${groupIndex}`}
                className={`flex flex-col gap-1 ${
                  groupIndex > 0 ? (collapsed ? 'border-t border-line pt-4' : 'border-t border-line pt-4') : ''
                }`}
              >
                {group.title && !collapsed && (
                  <p className="px-3 pb-1 text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">
                    {group.title}
                  </p>
                )}
                {group.items.map((item) => {
                  const badgeCount = badgeCounts?.[item.to] ?? 0;
                  const link = (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      end={item.end ?? item.to === '/'}
                      aria-label={collapsed ? item.label : undefined}
                      className={({ isActive }) =>
                        `relative flex min-h-11 items-center gap-3 rounded-lg text-[14px] font-medium transition-colors duration-150 ${
                          collapsed ? 'w-11 justify-center' : 'w-full px-3'
                        } ${isActive ? 'bg-primary-soft text-primary' : 'text-muted hover:bg-surface-2 hover:text-ink'}`
                      }
                    >
                      <item.icon size={20} strokeWidth={2} aria-hidden="true" />
                      {collapsed ? (
                        <NavBadgeDot count={badgeCount} />
                      ) : (
                        <>
                          {item.label}
                          <NavBadge count={badgeCount} />
                        </>
                      )}
                    </NavLink>
                  );

                  if (!collapsed) return link;

                  return (
                    <Tooltip key={item.to}>
                      <TooltipTrigger asChild>{link}</TooltipTrigger>
                      <TooltipContent side="right">{item.label}</TooltipContent>
                    </Tooltip>
                  );
                })}
              </div>
            ))}
          </nav>

          {userName && (
            <div
              className={`mt-auto flex items-center gap-2 border-t border-line pt-4 ${
                collapsed ? 'flex-col px-2' : 'justify-between px-5'
              }`}
            >
              {!collapsed && (
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-[14px] font-medium text-ink">{userName}</span>
                  {roleLabel && <span className="truncate text-[11px] text-muted">{roleLabel}</span>}
                </span>
              )}
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

        <div className="flex min-h-dvh flex-1 flex-col lg:h-dvh lg:min-w-0 lg:flex-1 lg:overflow-y-auto">
          <div className="sticky top-0 z-20 hidden bg-bg lg:block">
            <DesktopTopBar
              navItems={navItems}
              badgeCounts={badgeCounts}
              userName={userName}
              roleLabel={roleLabel}
              onLogout={onLogout}
            />
          </div>
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
                    end={item.end ?? item.to === '/'}
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
                        <span className="relative">
                          <item.icon
                            size={20}
                            strokeWidth={2}
                            aria-hidden="true"
                            className={isActive ? 'text-primary' : 'text-muted'}
                          />
                          {(badgeCounts?.[item.to] ?? 0) > 0 && (
                            <span
                              aria-hidden="true"
                              className="absolute -right-1.5 -top-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-ink"
                            >
                              {(badgeCounts?.[item.to] ?? 0) > 99 ? '99+' : badgeCounts?.[item.to]}
                            </span>
                          )}
                        </span>
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
    </TooltipProvider>
  );
}
