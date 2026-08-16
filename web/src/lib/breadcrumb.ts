import type { BreadcrumbItemDef } from '../components/ui/Breadcrumb';

/** Label segmen path level-1 — sinkron dengan rute di `App.tsx` & item `nav.ts`. */
const SEGMENT_LABEL: Record<string, string> = {
  data: 'Data',
  absensi: 'Absensi',
  izin: 'Izin',
  notifikasi: 'Notifikasi',
  profil: 'Profil',
  pengaturan: 'Pengaturan',
  admin: 'Sekolah',
  tagihan: 'Tagihan',
  kehadiran: 'Kehadiran',
  checkin: 'Check-in',
  scan: 'Scan',
  jurnal: 'Jurnal',
  monitoring: 'Monitoring',
  pengumuman: 'Pengumuman',
  kedisiplinan: 'Kedisiplinan',
  'pelanggaran-saya': 'Pelanggaran Saya',
  'izin-saya': 'Izin Saya',
  'izin-siswa': 'Izin Siswa',
  'qr-saya': 'QR Saya',
  'izin-keluar': 'Izin Keluar',
  terlambat: 'Terlambat',
  gerbang: 'Gerbang',
  jadwal: 'Jadwal',
  nilai: 'Nilai',
  'nilai-saya': 'Nilai Saya',
  'bintang-saya': 'Bintang Saya',
  konseling: 'Konseling',
  pengganti: 'Pengganti',
};

/** Label segmen level-2, dikelompokkan per induk (mis. `/data/siswa`). */
const CHILD_LABEL: Record<string, Record<string, string>> = {
  data: {
    siswa: 'Siswa',
    rombel: 'Rombel',
    guru: 'Guru',
    pegawai: 'Pegawai',
    mapel: 'Mapel',
    jadwal: 'Jadwal',
    jam: 'Jam',
    ruangan: 'Ruangan',
    tugas: 'Tugas',
    import: 'Import',
    'import-dapodik': 'Import Dapodik',
  },
  admin: {
    sekolah: 'Daftar Sekolah',
    new: 'Tambah Sekolah',
    outbox: 'Outbox',
    plans: 'Plan & Harga',
    minat: 'Minat Sekolah',
    pengumuman: 'Pengumuman Platform',
    pendapatan: 'Pendapatan',
  },
  izin: { rekap: 'Rekap', persetujuan: 'Persetujuan' },
  monitoring: { rekap: 'Rekap Ketertiban' },
  kedisiplinan: { rekap: 'Rekap', surat: 'Surat', pengaturan: 'Pengaturan' },
  'izin-siswa': { dispensasi: 'Dispensasi Keluar', terlambat: 'Terlambat', rekap: 'Rekap' },
  nilai: { input: 'Input', rekap: 'Rekap', rapor: 'Rapor' },
  pengganti: { 'untuk-saya': 'Untuk Saya', semua: 'Semua' },
  pengaturan: { 'hak-akses': 'Hak Akses' },
};

/** ID (UUID/angka) — tidak pernah dijadikan label, ditandai "Detail". */
function isIdSegment(segment: string): boolean {
  return /^[0-9a-f-]{8,}$/i.test(segment) || /^\d+$/.test(segment);
}

function humanize(segment: string): string {
  return segment
    .split('-')
    .map((w) => (w.length === 0 ? w : w.charAt(0).toUpperCase() + w.slice(1)))
    .join(' ');
}

/**
 * Turunkan breadcrumb dari path saat ini (docs/10 §5) — dipakai top bar
 * desktop `AppShell`. Rute yang belum ada di kamus (fitur baru) tetap
 * tampil lewat fallback humanize alih-alih hilang/pecah.
 */
export function getBreadcrumbItems(pathname: string): BreadcrumbItemDef[] {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 0) return [{ label: 'Beranda' }];

  const items: BreadcrumbItemDef[] = [];
  let pathSoFar = '';
  const [first, second] = segments;

  segments.forEach((segment, i) => {
    pathSoFar += `/${segment}`;
    const isLast = i === segments.length - 1;

    let label: string;
    if (i === 0) {
      label = SEGMENT_LABEL[segment] ?? humanize(segment);
    } else if (i === 1 && CHILD_LABEL[first]?.[second]) {
      label = CHILD_LABEL[first][second];
    } else if (isIdSegment(segment)) {
      label = 'Detail';
    } else {
      label = humanize(segment);
    }

    items.push({ label, to: isLast ? undefined : pathSoFar });
  });

  return items;
}
