import { useState, type FormEvent } from 'react';
import { Globe } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { Dialog } from '../../components/ui/Dialog';
import { Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Tag } from '../../components/ui/Tag';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useCustomDomain, useDeleteCustomDomain, useSetCustomDomain, useVerifyCustomDomain } from './api';

/** Seksi /pengaturan (admin) — status domain sendiri, form isi domain, instruksi DNS, verifikasi & hapus. */
export function CustomDomainSection() {
  const { data, isLoading, isError, refetch } = useCustomDomain();
  const setDomain = useSetCustomDomain();
  const verify = useVerifyCustomDomain();
  const del = useDeleteCustomDomain();
  const { showToast } = useToast();
  const [domainInput, setDomainInput] = useState('');
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const domain = domainInput.trim().toLowerCase();
    if (!domain) return;
    setDomain.mutate(domain, {
      onSuccess: () => {
        showToast('Domain disimpan. Ikuti instruksi di bawah untuk mengaktifkannya.');
        setDomainInput('');
      },
    });
  }

  function handleVerify() {
    verify.mutate(undefined, {
      onSuccess: (result) => {
        if (result.verified) {
          showToast('Domain berhasil diverifikasi.');
        } else {
          showToast('DNS belum mengarah ke server. Coba lagi setelah propagasi selesai.', 'error');
        }
      },
    });
  }

  function handleDelete() {
    del.mutate(undefined, {
      onSuccess: () => {
        showToast('Domain sendiri dihapus. Sekolah kembali memakai subdomain default.');
        setConfirmDeleteOpen(false);
      },
    });
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Domain</p>
        <h2 className="text-[18px] font-semibold text-ink">Domain Sendiri</h2>
      </div>

      {isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat status domain." onRetry={() => refetch()} />
      ) : data ? (
        <Card className="flex flex-col gap-4">
          {data.domain && data.verified ? (
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Globe size={20} strokeWidth={2} className="text-muted" aria-hidden="true" />
                <span className="text-[14px] font-medium text-ink">{data.domain}</span>
              </div>
              <Tag variant="success">Aktif</Tag>
            </div>
          ) : data.pending_domain ? (
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Globe size={20} strokeWidth={2} className="text-muted" aria-hidden="true" />
                <span className="text-[14px] font-medium text-ink">{data.pending_domain}</span>
              </div>
              <Tag variant="warning">Menunggu verifikasi</Tag>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <Globe size={20} strokeWidth={2} className="text-muted" aria-hidden="true" />
              <span className="text-[14px] text-muted">Belum diatur — sekolah memakai subdomain default.</span>
            </div>
          )}

          {(data.pending_domain || (!data.domain && !data.pending_domain)) && (
            <div className="rounded-lg bg-surface-2 p-3">
              {data.instructions && <p className="text-[12px] text-muted">{data.instructions}</p>}
              {data.server_ip && (
                <p className="num mt-1 font-mono text-[13px] text-ink">
                  Arahkan A record ke {data.server_ip}
                </p>
              )}
            </div>
          )}

          {!data.domain && (
            <form onSubmit={handleSubmit} className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <Field label="Domain" htmlFor="custom-domain-input" className="flex-1">
                <Input
                  id="custom-domain-input"
                  placeholder="sekolahku.sch.id"
                  value={domainInput}
                  onChange={(e) => setDomainInput(e.target.value)}
                  required
                />
              </Field>
              <Button type="submit" variant="secondary" loading={setDomain.isPending}>
                {data.pending_domain ? 'Ganti Domain' : 'Simpan Domain'}
              </Button>
            </form>
          )}
          {setDomain.isError && (
            <p className="text-[12px] text-danger">
              {setDomain.error instanceof ApiError ? setDomain.error.message : 'Gagal menyimpan domain.'}
            </p>
          )}

          {(data.pending_domain || data.domain) && (
            <div className="flex flex-wrap gap-2 border-t border-line pt-3">
              {!data.verified && (
                <Button type="button" loading={verify.isPending} onClick={handleVerify}>
                  Cek &amp; Verifikasi Sekarang
                </Button>
              )}
              <Button type="button" variant="danger" onClick={() => setConfirmDeleteOpen(true)}>
                Hapus Domain
              </Button>
            </div>
          )}
          {verify.isError && (
            <p className="text-[12px] text-danger">
              {verify.error instanceof ApiError ? verify.error.message : 'Gagal memverifikasi domain.'}
            </p>
          )}
        </Card>
      ) : null}

      <Dialog open={confirmDeleteOpen} onClose={() => setConfirmDeleteOpen(false)} title="Hapus domain sendiri?">
        <p className="text-[14px] text-ink">
          Sekolah akan kembali memakai subdomain default. Pengunjung yang membuka domain lama tidak akan terhubung
          lagi ke aplikasi ini.
        </p>
        {del.isError && (
          <p className="mt-3 text-[12px] text-danger">
            {del.error instanceof ApiError ? del.error.message : 'Gagal menghapus domain.'}
          </p>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setConfirmDeleteOpen(false)}>
            Batal
          </Button>
          <Button type="button" variant="danger" loading={del.isPending} onClick={handleDelete}>
            Hapus
          </Button>
        </div>
      </Dialog>
    </div>
  );
}
