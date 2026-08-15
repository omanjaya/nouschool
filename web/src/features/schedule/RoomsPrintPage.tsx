import { Link } from 'react-router-dom';
import { ChevronLeft, DoorOpen, Printer } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { roomQrImageUrl, useRooms } from './api';

/**
 * /data/ruangan/cetak — halaman print-friendly: satu kartu QR per ruangan.
 * AppShell (sidebar/bottom nav) disembunyikan lewat utility `print:hidden`
 * (lihat components/ui/AppShell.tsx); style tag lokal di sini hanya untuk
 * detail layout kartu saat cetak (grid, ukuran halaman, hindari kartu terpotong).
 */
export function RoomsPrintPage() {
  const { data: rooms, isLoading, isError, refetch } = useRooms();

  return (
    <div className="mx-auto max-w-[900px] px-5 py-6">
      <style>{`
        @media print {
          @page { margin: 12mm; }
          .qr-print-card { break-inside: avoid; }
        }
      `}</style>

      <div className="mb-6 flex items-start justify-between gap-3 print:hidden">
        <div>
          <Link
            to="/data/ruangan"
            className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
          >
            <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
            Ruangan
          </Link>
          <h1 className="text-[21px] font-semibold text-ink">Cetak Semua QR Ruangan</h1>
          <p className="text-[12px] text-muted">Tempel satu kartu di tiap ruangan untuk absensi scan QR.</p>
        </div>
        <Button onClick={() => window.print()} disabled={!rooms || rooms.length === 0}>
          <Printer size={16} strokeWidth={2} aria-hidden="true" />
          Cetak
        </Button>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
          <Skeleton className="h-56 w-full" />
          <Skeleton className="h-56 w-full" />
          <Skeleton className="h-56 w-full" />
        </div>
      ) : isError ? (
        <ErrorState message="Gagal memuat daftar ruangan." onRetry={() => refetch()} />
      ) : rooms && rooms.length === 0 ? (
        <EmptyState icon={DoorOpen} message="Belum ada ruangan untuk dicetak." />
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 print:grid-cols-2">
          {rooms?.map((room) => (
            <div
              key={room.id}
              className="qr-print-card flex flex-col items-center gap-3 rounded-lg border border-line p-5 text-center"
            >
              <img
                src={roomQrImageUrl(room.id)}
                alt={`QR ruangan ${room.name}`}
                className="h-40 w-40 object-contain"
              />
              <p className="text-[18px] font-semibold text-ink">{room.name}</p>
              <p className="text-[12px] text-muted">Scan saat masuk kelas — NouSchool</p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
