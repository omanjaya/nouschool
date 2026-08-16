import { useSearchParams } from 'react-router-dom';
import { BadgeCheck, ShieldAlert } from 'lucide-react';
import { BrandMark } from '../branding/BrandMark';
import { Skeleton } from '../../components/ui/Skeleton';
import { STUDENT_LEAVE_TYPE_LABEL } from './format';
import { useLeaveVerify } from './api';
import { formatDate, formatDateRange } from '../../lib/date';
import type { StudentLeaveType } from '../../lib/types';

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3 border-b border-line py-2 last:border-b-0">
      <span className="text-[12px] text-muted">{label}</span>
      <span className="text-[14px] font-medium text-ink">{value}</span>
    </div>
  );
}

/**
 * Halaman publik `/verifikasi-surat` (host TENANT), layar penuh tanpa AppShell —
 * sibling `/aktivasi`. Dibuka dari scan QR di surat izin PDF; tidak butuh login,
 * backend mengesahkan lewat `verify_token` sekali-lihat (bukan sesi).
 */
export function LeaveVerifyPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';
  const { data, isLoading, isError } = useLeaveVerify(token);

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-bg px-5 py-10">
      <BrandMark />
      <div className="w-full max-w-[420px]">
        {!token ? (
          <InvalidCard message="Tautan verifikasi tidak lengkap — token surat tidak ditemukan." />
        ) : isLoading ? (
          <div className="flex flex-col gap-3">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : isError || !data || !data.valid ? (
          <InvalidCard message="Surat tidak ditemukan atau tidak sah. Pastikan tautan/QR berasal dari surat izin asli." />
        ) : (
          <ValidCard result={data} />
        )}
      </div>
    </div>
  );
}

function ValidCard({
  result,
}: {
  result: {
    letter_number?: string;
    student_name?: string;
    class_name?: string;
    type?: StudentLeaveType;
    date_start?: string;
    date_end?: string;
    issued_at?: string;
  };
}) {
  return (
    <div className="flex flex-col gap-4 rounded-lg border border-st-hadir-line bg-st-hadir-line p-5">
      <div className="flex flex-col items-center gap-2 text-center">
        <BadgeCheck size={28} strokeWidth={2} className="text-st-hadir" aria-hidden="true" />
        <p className="text-[16px] font-semibold text-st-hadir">Surat Izin Sah</p>
        <p className="text-[12px] text-muted">Surat ini tercatat resmi di sistem sekolah.</p>
      </div>
      <div className="rounded-lg bg-surface px-4 py-2">
        {result.letter_number && <DetailRow label="Nomor Surat" value={result.letter_number} />}
        {result.student_name && <DetailRow label="Nama Siswa" value={result.student_name} />}
        {result.class_name && <DetailRow label="Rombel" value={result.class_name} />}
        {result.type && <DetailRow label="Jenis" value={STUDENT_LEAVE_TYPE_LABEL[result.type]} />}
        {result.date_start && result.date_end && (
          <DetailRow label="Tanggal Izin" value={formatDateRange(result.date_start, result.date_end)} />
        )}
        {result.issued_at && <DetailRow label="Diterbitkan" value={formatDate(result.issued_at)} />}
      </div>
    </div>
  );
}

function InvalidCard({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-line bg-danger-soft p-6 text-center">
      <ShieldAlert size={28} strokeWidth={2} className="text-danger" aria-hidden="true" />
      <p className="text-[16px] font-semibold text-danger">Surat Tidak Sah</p>
      <p className="text-[13px] text-ink">{message}</p>
    </div>
  );
}
