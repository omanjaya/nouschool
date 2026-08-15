import { useEffect, useState, type FormEvent } from 'react';
import { Dialog } from '../../components/ui/Dialog';
import { Button } from '../../components/ui/Button';
import { Field, Input, Textarea } from '../../components/ui/Field';
import { ApiError } from '../../lib/api';
import { todayISODate } from '../../lib/date';
import { useCreateAnnouncement, useUpdateAnnouncement } from './api';
import type { Announcement, AnnouncementInput } from '../../lib/types';

interface AnnouncementFormDialogProps {
  open: boolean;
  onClose: () => void;
  announcement?: Announcement;
  onSaved: (message: string) => void;
}

const EMPTY: AnnouncementInput = { title: '', body: '', starts_at: todayISODate(), ends_at: todayISODate() };

interface FormErrors {
  title?: string;
  body?: string;
  ends_at?: string;
}

/** Dialog tambah/ubah pengumuman — judul, isi, rentang tanggal aktif (docs/06 dashboard TV #3). */
export function AnnouncementFormDialog({ open, onClose, announcement, onSaved }: AnnouncementFormDialogProps) {
  const isEdit = Boolean(announcement);
  const [form, setForm] = useState<AnnouncementInput>(EMPTY);
  const [errors, setErrors] = useState<FormErrors>({});
  const createAnnouncement = useCreateAnnouncement();
  const updateAnnouncement = useUpdateAnnouncement(announcement?.id ?? '');
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
      { onSuccess: () => { onSaved(isEdit ? 'Pengumuman diperbarui.' : 'Pengumuman ditambahkan.'); onClose(); } },
    );
  }

  return (
    <Dialog open={open} onClose={onClose} title={isEdit ? 'Ubah Pengumuman' : 'Tambah Pengumuman'}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Field label="Judul" htmlFor="announcement-title" error={errors.title}>
          <Input
            id="announcement-title"
            value={form.title}
            onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
          />
        </Field>

        <Field label="Isi pengumuman" htmlFor="announcement-body" error={errors.body}>
          <Textarea
            id="announcement-body"
            value={form.body}
            onChange={(e) => setForm((f) => ({ ...f, body: e.target.value }))}
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Tanggal mulai" htmlFor="announcement-starts">
            <Input
              id="announcement-starts"
              type="date"
              value={form.starts_at}
              onChange={(e) => setForm((f) => ({ ...f, starts_at: e.target.value }))}
            />
          </Field>
          <Field label="Tanggal selesai" htmlFor="announcement-ends" error={errors.ends_at}>
            <Input
              id="announcement-ends"
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
