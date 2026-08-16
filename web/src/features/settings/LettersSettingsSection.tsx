import { useEffect, useState } from 'react';
import { Card } from '../../components/ui/Card';
import { Field, Textarea } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useLettersSettings, useUpdateLettersSettings } from './api';

/** Seksi "Catatan Kaki Surat" di /pengaturan (admin) — teks tetap di bagian bawah Surat Peringatan & Surat Izin. */
export function LettersSettingsSection() {
  const { data, isLoading, isError, refetch } = useLettersSettings();
  const update = useUpdateLettersSettings();
  const { showToast } = useToast();

  const [spFooterNote, setSpFooterNote] = useState('');
  const [leaveFooterNote, setLeaveFooterNote] = useState('');

  useEffect(() => {
    if (data) {
      setSpFooterNote(data.sp_footer_note);
      setLeaveFooterNote(data.leave_footer_note);
    }
  }, [data]);

  function handleSave() {
    update.mutate(
      { sp_footer_note: spFooterNote, leave_footer_note: leaveFooterNote },
      { onSuccess: () => showToast('Catatan kaki surat disimpan.') },
    );
  }

  if (isLoading) {
    return <Skeleton className="h-52 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat catatan kaki surat." onRetry={() => refetch()} />;
  }

  return (
    <Card className="flex flex-col gap-4">
      <Field label="Surat Peringatan" htmlFor="sp-footer-note">
        <Textarea
          id="sp-footer-note"
          value={spFooterNote}
          onChange={(e) => setSpFooterNote(e.target.value)}
          rows={3}
          placeholder="Mis. Surat ini berlaku sejak diterbitkan..."
        />
      </Field>

      <Field label="Surat Izin" htmlFor="leave-footer-note">
        <Textarea
          id="leave-footer-note"
          value={leaveFooterNote}
          onChange={(e) => setLeaveFooterNote(e.target.value)}
          rows={3}
          placeholder="Mis. Surat ini sah tanpa tanda tangan basah..."
        />
      </Field>

      {update.isError && (
        <p className="text-[12px] text-danger">
          {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan catatan kaki surat.'}
        </p>
      )}

      <Button onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan
      </Button>
    </Card>
  );
}
