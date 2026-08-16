import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronLeft, Info, Lock, ShieldAlert } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { useMe } from '../auth/api';
import { ApiError } from '../../lib/api';
import { ROLE_LABEL } from '../../lib/roles';
import { useRolePermissions, useUpdateRolePermissions } from './api';
import type { RolePermissionOverridesInput, RolePermissionOverrideValue } from '../../lib/types';
import { PAGE_WIDE } from '../../components/ui/page';

/** Role terkunci server (selalu punya semua hak akses) — kolomnya tampil read-only, tidak pernah dikirim di payload. */
const LOCKED_ROLE = 'admin_sekolah';
/** Permission terkunci server untuk SEMUA role (mis. hanya admin sekolah boleh kelola pengaturan). */
const LOCKED_PERMISSION = 'settings:manage';

type LocalMatrix = Record<string, Record<string, RolePermissionOverrideValue>>;

/**
 * /pengaturan/hak-akses — matrix hak akses per role (Fase 15 GAP 2, admin only).
 * Baris = permission, kolom = role. Sel = checkbox; nilai efektif = override
 * ?? default. Simpan hanya mengirim SEL YANG BERUBAH sesi ini (delta) —
 * `null` berarti "kembali ke bawaan" (docs/02-identity.md RBAC).
 */
export function RolePermissionsPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const { data, isLoading, isError, refetch } = useRolePermissions();
  const updateMutation = useUpdateRolePermissions();
  const { showToast } = useToast();
  const [matrix, setMatrix] = useState<LocalMatrix>({});

  // Sinkronkan state lokal setiap kali data server berubah (muat awal ATAU
  // setelah Simpan sukses — `useUpdateRolePermissions` menulis balik ke cache
  // query yang sama) supaya titik "diubah dari bawaan" selalu akurat.
  useEffect(() => {
    if (!data) return;
    const next: LocalMatrix = {};
    data.roles.forEach((role) => {
      if (role === LOCKED_ROLE) return;
      next[role] = {};
      data.permissions.forEach((perm) => {
        next[role][perm.key] = data.overrides[role]?.[perm.key] ?? null;
      });
    });
    setMatrix(next);
  }, [data]);

  const hasChanges = useMemo(() => {
    if (!data) return false;
    return data.roles.some((role) => {
      if (role === LOCKED_ROLE) return false;
      return data.permissions.some((perm) => {
        if (perm.key === LOCKED_PERMISSION) return false;
        const current = matrix[role]?.[perm.key] ?? null;
        const original = data.overrides[role]?.[perm.key] ?? null;
        return current !== original;
      });
    });
  }, [data, matrix]);

  if (meLoading) {
    return (
      <div className={`${PAGE_WIDE} flex flex-col gap-4`}>
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-52 w-full" />
      </div>
    );
  }

  if (!me || me.role !== 'admin_sekolah') {
    return (
      <div className={PAGE_WIDE}>
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  function toggle(role: string, permKey: string) {
    if (!data || role === LOCKED_ROLE || permKey === LOCKED_PERMISSION) return;
    const defaultVal = data.defaults[role]?.[permKey] ?? false;
    const current = matrix[role]?.[permKey] ?? null;
    const effective = current ?? defaultVal;
    const next = !effective;
    setMatrix((prev) => ({
      ...prev,
      [role]: { ...prev[role], [permKey]: next === defaultVal ? null : next },
    }));
  }

  function resetRole(role: string) {
    if (!data) return;
    setMatrix((prev) => {
      const reset: Record<string, RolePermissionOverrideValue> = {};
      data.permissions.forEach((perm) => {
        reset[perm.key] = null;
      });
      return { ...prev, [role]: reset };
    });
  }

  function handleSave() {
    if (!data) return;
    const payload: RolePermissionOverridesInput = {};
    data.roles.forEach((role) => {
      if (role === LOCKED_ROLE) return;
      const roleDelta: Record<string, RolePermissionOverrideValue> = {};
      let changed = false;
      data.permissions.forEach((perm) => {
        if (perm.key === LOCKED_PERMISSION) return;
        const current = matrix[role]?.[perm.key] ?? null;
        const original = data.overrides[role]?.[perm.key] ?? null;
        if (current !== original) {
          roleDelta[perm.key] = current;
          changed = true;
        }
      });
      if (changed) payload[role] = roleDelta;
    });
    if (Object.keys(payload).length === 0) return;

    updateMutation.mutate(payload, {
      onSuccess: () => showToast('Hak akses disimpan.'),
      onError: (err) => showToast(err instanceof ApiError ? err.message : 'Gagal menyimpan hak akses.', 'error'),
    });
  }

  return (
    <div className={`${PAGE_WIDE} flex flex-col gap-6`}>
      <div>
        <Link
          to="/pengaturan"
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Pengaturan
        </Link>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Pengaturan</p>
        <h1 className="text-[21px] font-semibold text-ink">Hak Akses</h1>
      </div>

      <div className="flex items-start gap-2 rounded-lg border border-line bg-surface-2 px-4 py-3">
        <Info size={16} strokeWidth={2} className="mt-0.5 shrink-0 text-muted" aria-hidden="true" />
        <p className="text-[13px] text-muted">
          Mengubah hak akses berlaku ke semua pengguna role tsb di sekolah ini.
        </p>
      </div>

      {isLoading ? (
        <div className="flex flex-col gap-2">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : isError || !data ? (
        <ErrorState message="Gagal memuat hak akses." onRetry={() => refetch()} />
      ) : data.permissions.length === 0 ? (
        <EmptyState icon={ShieldAlert} message="Belum ada daftar permission untuk ditampilkan." />
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border border-line">
            <table className="w-full min-w-[640px] border-collapse text-[13px]">
              <thead>
                <tr className="bg-surface-2 text-left text-[11px] font-semibold uppercase tracking-[0.05em] text-muted">
                  <th className="sticky left-0 z-10 bg-surface-2 px-3 py-2">Hak Akses</th>
                  {data.roles.map((role) => {
                    const locked = role === LOCKED_ROLE;
                    const roleHasChanges = !locked && data.permissions.some((perm) => (matrix[role]?.[perm.key] ?? null) !== null);
                    return (
                      <th key={role} className="px-3 py-2 text-center normal-case">
                        <div className="flex flex-col items-center gap-1">
                          <span className="inline-flex items-center gap-1 text-[12px] font-semibold text-ink">
                            {locked && <Lock size={12} strokeWidth={2} className="text-muted" aria-hidden="true" />}
                            {ROLE_LABEL[role] ?? role}
                          </span>
                          {!locked && (
                            <button
                              type="button"
                              onClick={() => resetRole(role)}
                              disabled={!roleHasChanges}
                              className="text-[11px] font-medium text-primary hover:opacity-80 disabled:cursor-not-allowed disabled:text-muted disabled:opacity-50"
                            >
                              Reset bawaan
                            </button>
                          )}
                        </div>
                      </th>
                    );
                  })}
                </tr>
              </thead>
              <tbody>
                {data.permissions.map((perm) => {
                  const permLocked = perm.key === LOCKED_PERMISSION;
                  return (
                    <tr key={perm.key} className="border-t border-line">
                      <td className="sticky left-0 z-10 bg-surface px-3 py-2.5">
                        <p className="text-[13px] text-ink">{perm.label}</p>
                        <p className="font-mono text-[11px] text-muted">{perm.key}</p>
                      </td>
                      {data.roles.map((role) => {
                        const isAdminCol = role === LOCKED_ROLE;
                        const locked = isAdminCol || permLocked;
                        const defaultVal = data.defaults[role]?.[perm.key] ?? (isAdminCol ? true : false);
                        const overrideVal = isAdminCol ? null : (matrix[role]?.[perm.key] ?? null);
                        const effective = overrideVal ?? defaultVal;
                        const overridden = overrideVal !== null;
                        return (
                          <td key={role} className="px-3 py-2.5 text-center">
                            <span
                              className="relative inline-flex"
                              title={overridden ? 'Diubah dari bawaan' : undefined}
                            >
                              <input
                                type="checkbox"
                                checked={effective}
                                disabled={locked}
                                onChange={() => toggle(role, perm.key)}
                                aria-label={`${perm.label} — ${ROLE_LABEL[role] ?? role}`}
                                className="h-4 w-4 rounded border border-line accent-primary focus:outline-none focus:ring-2 focus:ring-primary disabled:cursor-not-allowed disabled:opacity-60"
                              />
                              {overridden && (
                                <span
                                  className="absolute -right-1 -top-1 h-1.5 w-1.5 rounded-full bg-primary"
                                  aria-hidden="true"
                                />
                              )}
                            </span>
                          </td>
                        );
                      })}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="flex justify-end">
            <Button onClick={handleSave} loading={updateMutation.isPending} disabled={!hasChanges}>
              Simpan Hak Akses
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
