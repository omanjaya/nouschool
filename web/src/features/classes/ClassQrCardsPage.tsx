import { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { ChevronLeft, IdCard, Printer, RotateCcw } from 'lucide-react';
import { Skeleton } from '../../components/ui/Skeleton';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useMe } from '../auth/api';
import { qrCardImageUrl, useGenerateQrCards, useRevokeQrCard } from '../attendance/api';
import { useClasses } from './api';
import type { StudentQrCard } from '../../lib/types';

/**
 * /data/rombel/:id/kartu-qr — halaman print-friendly: satu kartu QR per siswa
 * anggota rombel, ukuran konsisten ≈85×54mm (rasio kartu identitas) supaya
 * mudah dicetak & dipotong. Kartu digenerate saat halaman dibuka (POST
 * .../generate, idempotent) — bukan GET, karena rombel bisa punya anggota
 * baru yang belum pernah dibuatkan kartu (pola cetak QR ruangan di
 * RoomsPrintPage memakai GET sederhana karena ruangan sudah tentu punya QR
 * sejak dibuat, kartu siswa berbeda: bisa bertambah anggota kapan saja).
 */
export function ClassQrCardsPage() {
  const { id } = useParams<{ id: string }>();
  const { data: me } = useMe();
  const { data: classes } = useClasses();
  const schoolClass = useMemo(() => classes?.find((c) => c.id === id), [classes, id]);

  const { showToast } = useToast();
  const generate = useGenerateQrCards();
  const revoke = useRevokeQrCard();

  const [cards, setCards] = useState<StudentQrCard[] | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [cacheBust, setCacheBust] = useState<Record<string, number>>({});
  const [revokeTarget, setRevokeTarget] = useState<StudentQrCard | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoadError(false);
    generate.mutate(id, {
      onSuccess: (data) => setCards(data),
      onError: () => setLoadError(true),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  function handleRevokeConfirm() {
    if (!revokeTarget) return;
    revoke.mutate(revokeTarget.student_id, {
      onSuccess: (newCard) => {
        setCards((prev) => (prev ? prev.map((c) => (c.student_id === newCard.student_id ? newCard : c)) : prev));
        setCacheBust((prev) => ({ ...prev, [newCard.student_id]: Date.now() }));
        showToast('Kartu lama tidak berlaku. Kartu baru siap dicetak.');
        setRevokeTarget(null);
      },
      onError: (err) => {
        showToast(err instanceof ApiError ? err.message : 'Gagal mencabut kartu.', 'error');
      },
    });
  }

  const schoolName = me?.school?.name ?? '';

  return (
    <div className="mx-auto max-w-[1000px] px-5 py-6">
      <style>{`
        @media print {
          @page { margin: 12mm; }
          .qr-print-card { break-inside: avoid; }
        }
      `}</style>

      <div className="mb-6 flex items-start justify-between gap-3 print:hidden">
        <div>
          <Link
            to={id ? `/data/rombel/${id}` : '/data/rombel'}
            className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
          >
            <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
            {schoolClass?.name ?? 'Rombel'}
          </Link>
          <h1 className="text-[21px] font-semibold text-ink">Kartu QR Siswa</h1>
          <p className="text-[12px] text-muted">
            Cetak lalu potong per kartu — tempel di kartu pelajar untuk absen scan QR.
          </p>
        </div>
        <Button onClick={() => window.print()} disabled={!cards || cards.length === 0}>
          <Printer size={16} strokeWidth={2} aria-hidden="true" />
          Cetak
        </Button>
      </div>

      {generate.isPending && !cards ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Skeleton className="h-[54mm] w-full" />
          <Skeleton className="h-[54mm] w-full" />
          <Skeleton className="h-[54mm] w-full" />
          <Skeleton className="h-[54mm] w-full" />
        </div>
      ) : loadError ? (
        <ErrorState
          message="Gagal membuat kartu QR."
          onRetry={() => {
            if (!id) return;
            setLoadError(false);
            generate.mutate(id, { onSuccess: (data) => setCards(data), onError: () => setLoadError(true) });
          }}
        />
      ) : cards && cards.length === 0 ? (
        <EmptyState icon={IdCard} message="Belum ada siswa di rombel ini." />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 print:grid-cols-2">
          {cards?.map((card) => (
            <div key={card.student_id} className="flex flex-col gap-2">
              <div
                className="qr-print-card flex items-center justify-between gap-3 rounded-lg border border-line p-3"
                style={{ width: '100%', maxWidth: '85mm', aspectRatio: '85 / 54' }}
              >
                <div className="flex min-w-0 flex-1 flex-col justify-center gap-1">
                  <p className="truncate text-[9px] font-semibold uppercase tracking-[0.08em] text-muted">
                    {schoolName}
                  </p>
                  <p className="truncate text-[14px] font-semibold text-ink">{card.name}</p>
                  <p className="num text-[11px] text-muted">{card.nis}</p>
                  <p className="truncate text-[11px] text-muted">{schoolClass?.name ?? ''}</p>
                </div>
                <img
                  src={qrCardImageUrl(card.student_id, cacheBust[card.student_id])}
                  alt={`Kartu QR ${card.name}`}
                  className="h-[34mm] w-[34mm] shrink-0 object-contain"
                />
              </div>
              <Button variant="secondary" onClick={() => setRevokeTarget(card)} className="self-start print:hidden">
                <RotateCcw size={16} strokeWidth={2} aria-hidden="true" />
                Cabut &amp; Ganti
              </Button>
            </div>
          ))}
        </div>
      )}

      <Dialog open={revokeTarget !== null} onClose={() => setRevokeTarget(null)} title="Cabut kartu lama?">
        <p className="text-[14px] text-ink">
          Kartu lama tidak berlaku. {revokeTarget?.name} akan mendapat kartu QR baru — cetak ulang kartunya.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setRevokeTarget(null)}>
            Batal
          </Button>
          <Button variant="danger" loading={revoke.isPending} onClick={handleRevokeConfirm}>
            Cabut &amp; Ganti
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
