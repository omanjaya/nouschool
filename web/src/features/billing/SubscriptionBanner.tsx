import { Link } from 'react-router-dom';
import { formatDate } from '../../lib/date';
import type { Me } from '../../lib/types';

/**
 * Banner status langganan global (docs/09-billing.md lifecycle) — tampil di
 * atas konten untuk role sekolah host tenant mana pun saat status `grace`
 * (kuning, peringatan) atau `readonly` (merah, mode baca saja). Ditautkan ke
 * `/tagihan` hanya untuk role yang bisa mengelolanya (admin & kepsek);
 * role lain tetap melihat pesannya tanpa tautan.
 *
 * Sengaja TIDAK dirender untuk role `display` / rute `/tv` — App.tsx hanya
 * memasang komponen ini di dalam `AuthenticatedShell`, dan role `display`
 * dialihkan ke `/tv` (rute fullscreen terpisah tanpa shell) sebelum shell
 * ini pernah dirender.
 */
export function SubscriptionBanner({ me }: { me: Me }) {
  const subscription = me.subscription;
  if (!subscription || (subscription.status !== 'grace' && subscription.status !== 'readonly')) return null;

  const canManageBilling = me.role === 'admin_sekolah' || me.role === 'kepala_sekolah';

  if (subscription.status === 'readonly') {
    return (
      <div className="flex items-center justify-center gap-2 bg-danger px-4 py-2 text-center text-[12px] font-medium text-primary-ink">
        <span>Langganan berakhir — mode baca saja.</span>
        {canManageBilling && (
          <Link to="/tagihan" className="underline underline-offset-2">
            Perpanjang sekarang
          </Link>
        )}
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center gap-2 bg-st-terlambat-line px-4 py-2 text-center text-[12px] font-medium text-st-terlambat">
      <span>
        Langganan berakhir {formatDate(subscription.ends_on)}.
        {subscription.grace_until && ` Perpanjang sebelum ${formatDate(subscription.grace_until)} agar tetap bisa menginput data.`}
      </span>
      {canManageBilling && (
        <Link to="/tagihan" className="underline underline-offset-2">
          Perpanjang
        </Link>
      )}
    </div>
  );
}
