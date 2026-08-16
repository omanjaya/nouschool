import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { todayISODate } from '../../lib/date';
import { useCreatePlatformAnnouncement, useUpdatePlatformAnnouncement } from './api';
import type { PlatformAnnouncement, PlatformAnnouncementInput } from '../../lib/types';

interface PlatformAnnouncementFormDialogProps {
  open: boolean;
  onClose: () => void;
  announcement?: PlatformAnnouncement;
  onSaved: (message: string) => void;
}

const EMPTY: PlatformAnnouncementInput = { title: '', body: '', starts_at: todayISODate(), ends_at: todayISODate() };

interface FormErrors {
  title?: string;
  body?: string;
  ends_at?: string;
}

/**
 * Dialog tambah/ubah pengumuman platform (Fase 13 Gelombang 2 P5, docs/11 P5) — pola identik
 * `AnnouncementFormDialog` sekolah, target endpoint `/admin/platform-announcements`.
 */
export function PlatformAnnouncementFormDialog({
  open,
  onClose,
  announcement,
  onSaved,
}: PlatformAnnouncementFormDialogProps) {
  const isEdit = Boolean(announcement);
  const [form, setForm] = useState<PlatformAnnouncementInput>(EMPTY);
  const [errors, setErrors] = useState<FormErrors>({});
  const createAnnouncement = useCreatePlatformAnnouncement();
  const updateAnnouncement = useUpdatePlatformAnnouncement(announcement?.id ?? '');
  const mutation = isEdit ? updateAnnouncement : createAnnouncement;

  useEffect(() => {
    if (open) {
      setForm(
        announcement
          ? {
              title: announcement.title,
              body: announcement.body,
              starts_at: announcement.starts_at,
              ends_at: announcement.ends_at,
            }
          : EMPTY,
      );
      setErrors({});
      mutation.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, announcement]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const nextErrors: FormErrors = {};
    if (!form.title.trim()) nextErrors.title = 'Judul wajib diisi.';
    if (!form.body.trim()) nextErrors.body = 'Isi pengumuman wajib diisi.';
    if (form.ends_at < form.starts_at) nextErrors.ends_at = 'Tanggal selesai tidak boleh sebelum tanggal mulai.';
    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }
    setErrors({});
    mutation.mutate(
      { title: form.title.trim(), body: form.body.trim(), starts_at: form.starts_at, ends_at: form.ends_at },
      {
        onSuccess: () => {
          onSaved(isEdit ? 'Pengumuman diperbarui.' : 'Pengumuman ditambahkan.');
          onClose();
        },
      },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Pengumuman' : 'Tambah Pengumuman'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Judul" htmlFor="platform-announcement-title" error={errors.title}>
          <Input
            id="platform-announcement-title"
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
          />
        </Field>

        <Field label="Isi pengumuman" htmlFor="platform-announcement-body" error={errors.body}>
          <Textarea
            id="platform-announcement-body"
            value={form.body}
            onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Tanggal mulai" htmlFor="platform-announcement-starts">
            <Input
              id="platform-announcement-starts"
              type="date"
              value={form.starts_at}
              onChange={(e) => setForm((f) => ({ ...f, starts_at: e.target.value }))}
            />
          </Field>
          <Field label="Tanggal selesai" htmlFor="platform-announcement-ends" error={errors.ends_at}>
            <Input
              id="platform-announcement-ends"
              type="date"
              value={form.ends_at}
              onChange={(e) => setForm((f) => ({ ...f, ends_at: e.target.value }))}
            />
          </Field>
        </div>

        {mutation.isError && (
          <p className="text-[12px] text-danger">
            {mutation.error instanceof ApiError ? mutation.error.message : 'Gagal menyimpan pengumuman.'}
          </p>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            Batal
          </Button>
          <Button type="submit" loading={mutation.isPending}>
            Simpan
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
