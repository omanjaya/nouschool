import { AlertTriangle } from 'lucide-react';
import { Button } from './Button';

interface ErrorStateProps {
  message?: string;
  onRetry?: () => void;
}

/** Pesan apa yang salah + tombol coba lagi — WAJIB untuk setiap fetch yang bisa gagal. */
export function ErrorState({ message = 'Gagal memuat data. Coba lagi.', onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-10 text-center">
      <AlertTriangle size={24} strokeWidth={2} className="text-danger" aria-hidden="true" />
      <p className="text-[14px] text-muted">{message}</p>
      {onRetry && (
        <Button variant="secondary" onClick={onRetry}>
          Coba lagi
        </Button>
      )}
    </div>
  );
}
