import { useEffect, useState } from 'react';
import { LocateFixed } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Field, Input, Select } from '../../components/ui/Field';
import { Button } from '../../components/ui/Button';
import { Skeleton } from '../../components/ui/Skeleton';
import { ErrorState } from '../../components/ui/ErrorState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import { useAttendanceSettings, useUpdateAttendanceSettings } from './api';
import type { AttendanceMethod, AttendanceMode, AttendanceSettings } from '../../lib/types';

const MODE_HINT: Record<AttendanceMode, string> = {
  daily: 'Satu kali absen per hari untuk seluruh rombel — cocok untuk sekolah tanpa jadwal per jam.',
  per_subject: 'Absen per jam pelajaran mengikuti jadwal — guru mengisi tiap kali mengajar.',
};

const METHOD_OPTIONS: { value: AttendanceMethod; label: string; hint: string }[] = [
  { value: 'manual', label: 'Manual', hint: 'Guru mencatat kehadiran satu per satu di layar sesi absensi.' },
  { value: 'qr_card', label: 'Kartu QR', hint: 'Siswa membawa kartu QR yang dipindai guru saat masuk kelas.' },
  {
    value: 'self_checkin',
    label: 'Check-in Mandiri',
    hint: 'Siswa absen datang sendiri lewat lokasi HP dalam jendela waktu tertentu.',
  },
];

const GEOLOCATION_TIMEOUT_MS = 10_000;

interface FormState {
  mode: AttendanceMode;
  methods: Set<AttendanceMethod>;
  lat: string;
  lng: string;
  radiusM: string;
  openFrom: string;
  closeAt: string;
  lateAfterMin: string;
  editWindowHours: string;
}

function toFormState(data: AttendanceSettings): FormState {
  return {
    mode: data.mode,
    methods: new Set(data.methods),
    lat: data.self_checkin ? String(data.self_checkin.lat) : '',
    lng: data.self_checkin ? String(data.self_checkin.lng) : '',
    radiusM: data.self_checkin ? String(data.self_checkin.radius_m) : '150',
    openFrom: data.self_checkin?.open_from ?? '06:00',
    closeAt: data.self_checkin?.close_at ?? '07:30',
    lateAfterMin: String(data.late_after_min),
    editWindowHours: String(data.edit_window_hours),
  };
}

/** Seksi "Pengaturan Absensi" di /pengaturan (admin) — mode, metode input, & rule check-in mandiri. */
export function AttendanceSettingsSection() {
  const { data, isLoading, isError, refetch } = useAttendanceSettings();
  const update = useUpdateAttendanceSettings();
  const { showToast } = useToast();

  const [form, setForm] = useState<FormState | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [locating, setLocating] = useState(false);

  useEffect(() => {
    if (data) setForm(toFormState(data));
  }, [data]);

  function toggleMethod(method: AttendanceMethod) {
    setForm((prev) => {
      if (!prev) return prev;
      const next = new Set(prev.methods);
      if (next.has(method)) next.delete(method);
      else next.add(method);
      return { ...prev, methods: next };
    });
  }

  function handleUseMyLocation() {
    setFormError(null);
    if (!('geolocation' in navigator)) {
      setFormError('Perangkat ini tidak mendukung layanan lokasi.');
      return;
    }
    setLocating(true);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setLocating(false);
        setForm((prev) =>
          prev
            ? { ...prev, lat: String(position.coords.latitude), lng: String(position.coords.longitude) }
            : prev,
        );
      },
      () => {
        setLocating(false);
        setFormError('Gagal mengambil lokasi. Izinkan akses lokasi di browser.');
      },
      { enableHighAccuracy: true, timeout: GEOLOCATION_TIMEOUT_MS },
    );
  }

  function handleSave() {
    if (!form) return;
    setFormError(null);

    if (form.methods.size === 0) {
      setFormError('Pilih minimal satu metode absensi.');
      return;
    }

    const editWindowHours = Number(form.editWindowHours);
    if (!Number.isFinite(editWindowHours) || editWindowHours <= 0) {
      setFormError('Jendela edit absensi harus berupa angka jam lebih dari 0.');
      return;
    }

    const lateAfterMin = Number(form.lateAfterMin);
    if (!Number.isFinite(lateAfterMin) || lateAfterMin < 0) {
      setFormError('Toleransi terlambat harus berupa angka menit, 0 atau lebih.');
      return;
    }

    let selfCheckin: AttendanceSettings['self_checkin'] = null;
    if (form.methods.has('self_checkin')) {
      const lat = Number(form.lat);
      const lng = Number(form.lng);
      const radiusM = Number(form.radiusM);
      if (!form.lat || !form.lng || !Number.isFinite(lat) || !Number.isFinite(lng)) {
        setFormError('Isi titik lokasi sekolah (lat & lng) untuk check-in mandiri.');
        return;
      }
      if (!Number.isFinite(radiusM) || radiusM <= 0) {
        setFormError('Radius check-in harus lebih dari 0 meter.');
        return;
      }
      if (!form.openFrom || !form.closeAt) {
        setFormError('Isi jam buka & tutup jendela check-in.');
        return;
      }
      if (form.openFrom >= form.closeAt) {
        setFormError('Jam buka check-in harus lebih awal dari jam tutup.');
        return;
      }
      selfCheckin = { lat, lng, radius_m: radiusM, open_from: form.openFrom, close_at: form.closeAt };
    }

    const payload: AttendanceSettings = {
      mode: form.mode,
      methods: Array.from(form.methods),
      self_checkin: selfCheckin,
      edit_window_hours: editWindowHours,
      late_after_min: lateAfterMin,
    };

    update.mutate(payload, {
      onSuccess: () => showToast('Pengaturan absensi disimpan.'),
      onError: (err) => setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan pengaturan absensi.'),
    });
  }

  if (isLoading || !form) {
    return <Skeleton className="h-64 w-full" />;
  }

  if (isError || !data) {
    return <ErrorState message="Gagal memuat pengaturan absensi." onRetry={() => refetch()} />;
  }

  return (
    <div className="flex flex-col gap-4">
      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Mode Absensi</p>
          <p className="text-[12px] text-muted">Menentukan seberapa sering siswa diabsen dalam sehari.</p>
        </div>
        <Field label="Mode" hint={MODE_HINT[form.mode]}>
          <Select
            value={form.mode}
            onChange={(e) => setForm((prev) => (prev ? { ...prev, mode: e.target.value as AttendanceMode } : prev))}
          >
            <option value="daily">Harian</option>
            <option value="per_subject">Per Mata Pelajaran</option>
          </Select>
        </Field>
      </Card>

      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Metode Input</p>
          <p className="text-[12px] text-muted">Boleh mengaktifkan lebih dari satu metode sekaligus.</p>
        </div>
        <div className="flex flex-col gap-3">
          {METHOD_OPTIONS.map((opt) => (
            <label key={opt.value} className="flex cursor-pointer items-start gap-3">
              <input
                type="checkbox"
                checked={form.methods.has(opt.value)}
                onChange={() => toggleMethod(opt.value)}
                className="mt-0.5 h-4 w-4 shrink-0 accent-[var(--primary)]"
              />
              <span>
                <span className="block text-[14px] text-ink">{opt.label}</span>
                <span className="block text-[12px] text-muted">{opt.hint}</span>
              </span>
            </label>
          ))}
        </div>
      </Card>

      {form.methods.has('self_checkin') && (
        <Card className="flex flex-col gap-4">
          <div>
            <p className="text-[16px] font-semibold text-ink">Check-in Mandiri</p>
            <p className="text-[12px] text-muted">Titik lokasi sekolah, radius, & jendela waktu check-in.</p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Field label="Lintang (lat)" className="min-w-[140px] flex-1" htmlFor="checkin-lat">
              <Input
                id="checkin-lat"
                type="number"
                step="any"
                value={form.lat}
                onChange={(e) => setForm((prev) => (prev ? { ...prev, lat: e.target.value } : prev))}
              />
            </Field>
            <Field label="Bujur (lng)" className="min-w-[140px] flex-1" htmlFor="checkin-lng">
              <Input
                id="checkin-lng"
                type="number"
                step="any"
                value={form.lng}
                onChange={(e) => setForm((prev) => (prev ? { ...prev, lng: e.target.value } : prev))}
              />
            </Field>
          </div>

          <Button variant="secondary" onClick={handleUseMyLocation} loading={locating} className="self-start">
            <LocateFixed size={16} strokeWidth={2} aria-hidden="true" />
            Pakai Lokasi Saya
          </Button>

          <Field label="Radius (meter)" htmlFor="checkin-radius" className="max-w-[200px]">
            <Input
              id="checkin-radius"
              type="number"
              min={1}
              value={form.radiusM}
              onChange={(e) => setForm((prev) => (prev ? { ...prev, radiusM: e.target.value } : prev))}
            />
          </Field>

          <div className="flex flex-wrap gap-3">
            <Field label="Jam buka" className="min-w-[140px] flex-1" htmlFor="checkin-open">
              <Input
                id="checkin-open"
                type="time"
                value={form.openFrom}
                onChange={(e) => setForm((prev) => (prev ? { ...prev, openFrom: e.target.value } : prev))}
              />
            </Field>
            <Field label="Jam tutup" className="min-w-[140px] flex-1" htmlFor="checkin-close">
              <Input
                id="checkin-close"
                type="time"
                value={form.closeAt}
                onChange={(e) => setForm((prev) => (prev ? { ...prev, closeAt: e.target.value } : prev))}
              />
            </Field>
          </div>

          <Field label="Toleransi terlambat (menit)" htmlFor="checkin-late" className="max-w-[220px]">
            <Input
              id="checkin-late"
              type="number"
              min={0}
              value={form.lateAfterMin}
              onChange={(e) => setForm((prev) => (prev ? { ...prev, lateAfterMin: e.target.value } : prev))}
            />
          </Field>
        </Card>
      )}

      <Card className="flex flex-col gap-4">
        <div>
          <p className="text-[16px] font-semibold text-ink">Jendela Edit</p>
          <p className="text-[12px] text-muted">Setelah jendela ini lewat, hanya admin yang bisa mengubah absensi.</p>
        </div>
        <Field label="Jendela edit (jam)" htmlFor="edit-window" className="max-w-[220px]">
          <Input
            id="edit-window"
            type="number"
            min={1}
            value={form.editWindowHours}
            onChange={(e) => setForm((prev) => (prev ? { ...prev, editWindowHours: e.target.value } : prev))}
          />
        </Field>
      </Card>

      {formError && <p className="text-[12px] text-danger">{formError}</p>}

      <Button onClick={handleSave} loading={update.isPending} className="self-start">
        Simpan Pengaturan Absensi
      </Button>
    </div>
  );
}
