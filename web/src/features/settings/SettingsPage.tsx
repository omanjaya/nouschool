import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Image as ImageIcon, ShieldAlert, ShieldCheck } from 'lucide-react';
import { useMe } from '../auth/api';
import { usePublicContext } from '../branding/api';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { EmptyState } from '../../components/ui/EmptyState';
import { Card } from '../../components/ui/Card';
import { Field, Input } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { useBranding, useUpdateBranding, useUploadBrandingLogo } from './api';
import { ApiError } from '../../lib/api';
import { applyBrandColor } from '../../lib/color';
import { LeaveSettingsSection } from '../leave/LeaveSettingsSection';
import { AttendanceSettingsSection } from '../attendance/AttendanceSettingsSection';
import { GradingSettingsSection } from '../grading/GradingSettingsSection';
import { CustomDomainSection } from './CustomDomainSection';
import { LettersSettingsSection } from './LettersSettingsSection';
import { SecuritySettingsSection } from './SecuritySettingsSection';

const MAX_LOGO_BYTES = 2 * 1024 * 1024;

/**
 * Anchor nav sekunder khusus desktop (`lg`) — halaman ini menumpuk 6+ seksi
 * panjang, jadi kolom kiri sticky memberi jalan pintas lompat-ke-seksi tanpa
 * scroll panjang. Link biasa (`href="#id"` + `scroll-mt-6` di seksi) — tanpa
 * IntersectionObserver supaya tetap sederhana & stabil (tidak ada state aktif
 * yang bisa salah sinkron dengan posisi scroll).
 */
const SETTINGS_SECTIONS = [
  { id: 'branding', label: 'Branding' },
  { id: 'domain', label: 'Domain' },
  { id: 'absensi', label: 'Absensi' },
  { id: 'izin', label: 'Izin' },
  { id: 'penilaian', label: 'Penilaian' },
  { id: 'surat', label: 'Surat' },
  { id: 'keamanan', label: 'Keamanan' },
] as const;

/** /pengaturan — hanya admin_sekolah, area sekolah (host tenant). */
export function SettingsPage() {
  const { data: me, isLoading } = useMe();
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-52 w-full" />
      </div>
    );
  }

  if (!me || me.role !== 'admin_sekolah') {
    return (
      <div className="mx-auto max-w-[640px] px-5 py-6">
        <EmptyState icon={ShieldAlert} message="Anda tidak memiliki akses ke halaman ini." />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[640px] px-5 py-6 lg:max-w-[960px] lg:grid lg:grid-cols-[200px_1fr] lg:items-start lg:gap-8">
      <nav aria-label="Bagian pengaturan" className="hidden lg:sticky lg:top-6 lg:flex lg:flex-col lg:gap-1">
        {SETTINGS_SECTIONS.map((section) => (
          <a
            key={section.id}
            href={`#${section.id}`}
            className="rounded-lg px-3 py-2 text-[13px] font-medium text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
          >
            {section.label}
          </a>
        ))}
        {/*
          "Hak Akses" (Fase 15 GAP 2) bukan anchor di halaman ini — matrix
          role×permission cukup besar untuk jadi halaman tersendiri
          (`/pengaturan/hak-akses`), jadi item nav ini pakai `Link` router,
          bukan `href="#id"` seperti seksi lain di atas.
        */}
        <Link
          to="/pengaturan/hak-akses"
          className="rounded-lg px-3 py-2 text-[13px] font-medium text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
        >
          Hak Akses
        </Link>
      </nav>

      <div className="flex flex-col gap-8">
        <div id="branding" className="max-w-[680px] scroll-mt-6">
          <BrandingForm />
        </div>
        <div id="domain" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <CustomDomainSection />
        </div>
        <div id="absensi" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Absensi</p>
            <h2 className="text-[18px] font-semibold text-ink">Pengaturan Absensi</h2>
          </div>
          <AttendanceSettingsSection />
        </div>
        <div id="izin" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Izin</p>
            <h2 className="text-[18px] font-semibold text-ink">Pengaturan Izin Guru</h2>
          </div>
          <LeaveSettingsSection />
        </div>
        <div id="penilaian" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Penilaian</p>
            <h2 className="text-[18px] font-semibold text-ink">Pengaturan Penilaian</h2>
          </div>
          <GradingSettingsSection />
        </div>
        <div id="surat" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Surat</p>
            <h2 className="text-[18px] font-semibold text-ink">Catatan Kaki Surat</h2>
          </div>
          <LettersSettingsSection />
        </div>
        <div className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Hak Akses</p>
            <h2 className="text-[18px] font-semibold text-ink">Hak Akses per Role</h2>
          </div>
          <Card className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-[13px] text-muted">Atur permission yang dimiliki tiap role di sekolah ini.</p>
            <Button variant="secondary" onClick={() => navigate('/pengaturan/hak-akses')}>
              <ShieldCheck size={16} strokeWidth={2} aria-hidden="true" />
              Buka Hak Akses
            </Button>
          </Card>
        </div>
        <div id="keamanan" className="flex max-w-[680px] scroll-mt-6 flex-col gap-4 border-t border-line pt-6">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Keamanan</p>
            <h2 className="text-[18px] font-semibold text-ink">Pengaturan Keamanan</h2>
          </div>
          <SecuritySettingsSection />
        </div>
      </div>
    </div>
  );
}

function BrandingForm() {
  const { data: branding, isLoading, isError, refetch } = useBranding();
  const { data: context } = usePublicContext();
  const update = useUpdateBranding();
  const uploadLogo = useUploadBrandingLogo();
  const { showToast } = useToast();
  const [appName, setAppName] = useState('');
  const [primaryColor, setPrimaryColor] = useState('#0E6B4E');
  const [logoPreview, setLogoPreview] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (branding) {
      setAppName(branding.app_name);
      setPrimaryColor(branding.primary_color);
    }
  }, [branding]);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    update.mutate(
      { app_name: appName, primary_color: primaryColor },
      {
        onSuccess: () => {
          // Preview live warna: cukup terapkan begitu simpan sukses (bukan
          // saat drag color picker) — sederhana & tidak butuh logika revert.
          applyBrandColor(primaryColor);
          showToast('Pengaturan disimpan.');
        },
      },
    );
  }

  function handlePickLogo(file: File | null) {
    if (!file) return;
    if (file.size > MAX_LOGO_BYTES) {
      showToast('Ukuran logo maksimal 2MB.', 'error');
      return;
    }
    const objectUrl = URL.createObjectURL(file);
    setLogoPreview(objectUrl);
    uploadLogo.mutate(file, {
      onSuccess: () => showToast('Logo diperbarui.'),
      onError: () => setLogoPreview(null),
    });
  }

  const currentLogoUrl = context && !context.platform ? context.branding.logo_url : null;
  const displayedLogo = logoPreview ?? currentLogoUrl;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.1em] text-muted">Sekolah</p>
        <h1 className="text-[21px] font-semibold text-ink">Pengaturan</h1>
      </div>

      {isLoading ? (
        <Skeleton className="h-52 w-full" />
      ) : isError ? (
        <ErrorState message="Gagal memuat pengaturan tampilan." onRetry={() => refetch()} />
      ) : (
        <Card>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] font-medium text-muted">Logo sekolah</label>
              <div className="flex items-center gap-3">
                <div className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-line bg-surface-2">
                  {displayedLogo ? (
                    <img src={displayedLogo} alt="Logo sekolah" className="h-full w-full object-contain" />
                  ) : (
                    <ImageIcon size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
                  )}
                </div>
                <div className="flex flex-col gap-1">
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/png,image/jpeg"
                    className="hidden"
                    onChange={(e) => {
                      handlePickLogo(e.target.files?.[0] ?? null);
                      e.target.value = '';
                    }}
                  />
                  <Button
                    type="button"
                    variant="secondary"
                    loading={uploadLogo.isPending}
                    onClick={() => fileInputRef.current?.click()}
                    className="self-start"
                  >
                    Ganti Logo
                  </Button>
                  <span className="text-[12px] text-muted">PNG/JPG, maksimal 2MB.</span>
                </div>
              </div>
              {uploadLogo.isError && (
                <p className="text-[12px] text-danger">
                  {uploadLogo.error instanceof ApiError ? uploadLogo.error.message : 'Gagal mengunggah logo.'}
                </p>
              )}
            </div>

            <Field label="Nama aplikasi" htmlFor="app-name">
              <Input id="app-name" value={appName} onChange={(e) => setAppName(e.target.value)} required />
            </Field>

            <Field label="Warna utama" htmlFor="primary-color">
              <div className="flex items-center gap-3">
                <input
                  id="primary-color"
                  type="color"
                  value={primaryColor}
                  onChange={(e) => setPrimaryColor(e.target.value)}
                  className="h-11 w-14 cursor-pointer rounded-lg border border-line bg-surface p-1"
                />
                <span className="num text-[14px] text-ink">{primaryColor.toUpperCase()}</span>
              </div>
            </Field>

            {update.isError && (
              <p className="text-[12px] text-danger">
                {update.error instanceof ApiError ? update.error.message : 'Gagal menyimpan pengaturan.'}
              </p>
            )}

            <Button type="submit" loading={update.isPending} className="self-start">
              Simpan Pengaturan
            </Button>
          </form>
        </Card>
      )}
    </div>
  );
}
