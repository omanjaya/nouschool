import { useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { ChevronLeft, FileSpreadsheet, UploadCloud } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { StatTile } from '../../components/ui/StatTile';
import { ListRow } from '../../components/ui/ListRow';
import { Tag } from '../../components/ui/Tag';
import { EmptyState } from '../../components/ui/EmptyState';
import { useToast } from '../../components/ui/Toast';
import { ApiError } from '../../lib/api';
import type { ImportRow } from '../../lib/types';
import { IMPORT_ENTITY_LABEL, importTemplateUrl, useImportCommit, useImportPreview, type ImportEntity } from './api';

interface ImportWizardProps {
  entity: ImportEntity;
  /** Rute daftar untuk tombol kembali & tujuan setelah commit selesai. */
  backTo: string;
}

type Step = 'upload' | 'preview' | 'result';

const ACTION_LABEL: Record<ImportRow['action'], string> = {
  create: 'Baru',
  update: 'Update',
  error: 'Error',
};

function rowDisplayName(row: ImportRow): string {
  return row.data.nama ?? row.data.name ?? `Baris ${row.row}`;
}

function rowSubtitle(row: ImportRow): string {
  const entries = Object.entries(row.data).filter(([key]) => key !== 'nama' && key !== 'name');
  return entries
    .filter(([, value]) => value)
    .map(([, value]) => value)
    .join(' · ');
}

/** Wizard 3 langkah: upload → preview & validasi → commit. Dipakai untuk import siswa & guru. */
export function ImportWizard({ entity, backTo }: ImportWizardProps) {
  const [step, setStep] = useState<Step>('upload');
  const [file, setFile] = useState<File | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const preview = useImportPreview(entity);
  const commit = useImportCommit(entity);
  const { showToast } = useToast();
  const navigate = useNavigate();

  const label = IMPORT_ENTITY_LABEL[entity];

  function pickFile(f: File | null) {
    setFile(f);
    preview.reset();
  }

  function handleUpload() {
    if (!file) return;
    preview.mutate(file, {
      onSuccess: () => setStep('preview'),
    });
  }

  function handleCommit() {
    if (!preview.data) return;
    commit.mutate(preview.data.upload_id, {
      onSuccess: () => {
        showToast(`Import ${label} berhasil.`);
        setStep('result');
      },
    });
  }

  const validCount = preview.data ? preview.data.summary.create + preview.data.summary.update : 0;

  return (
    <div className="mx-auto flex max-w-[640px] flex-col gap-6 px-5 py-6">
      <div>
        <button
          type="button"
          onClick={() => navigate(backTo)}
          className="mb-3 inline-flex items-center gap-1 text-[12px] font-medium text-muted hover:text-ink"
        >
          <ChevronLeft size={16} strokeWidth={2} aria-hidden="true" />
          Kembali
        </button>
        <h1 className="text-[21px] font-semibold text-ink">Import Data {label === 'siswa' ? 'Siswa' : 'Guru'}</h1>
        <p className="text-[12px] text-muted">Langkah {step === 'upload' ? '1' : step === 'preview' ? '2' : '3'} dari 3</p>
      </div>

      {step === 'upload' && (
        <div className="flex flex-col gap-4">
          <a
            href={importTemplateUrl(entity)}
            className="inline-flex w-fit items-center gap-1.5 text-[12px] font-medium text-primary hover:opacity-80"
          >
            <FileSpreadsheet size={16} strokeWidth={2} aria-hidden="true" />
            Unduh template {label}
          </a>

          <div
            onDragOver={(e) => {
              e.preventDefault();
              setDragOver(true);
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragOver(false);
              const dropped = e.dataTransfer.files?.[0];
              if (dropped) pickFile(dropped);
            }}
            className={`flex flex-col items-center gap-3 rounded-lg border border-dashed px-6 py-10 text-center transition-colors duration-150 ${
              dragOver ? 'border-primary bg-primary-soft' : 'border-line'
            }`}
          >
            <UploadCloud size={24} strokeWidth={2} className="text-muted" aria-hidden="true" />
            <p className="text-[14px] text-ink">
              {file ? file.name : 'Seret file ke sini atau pilih dari perangkat'}
            </p>
            <p className="text-[12px] text-muted">Format .xlsx atau .csv</p>
            <input
              ref={fileInputRef}
              type="file"
              accept=".xlsx,.csv"
              className="hidden"
              onChange={(e) => pickFile(e.target.files?.[0] ?? null)}
            />
            <Button type="button" variant="secondary" onClick={() => fileInputRef.current?.click()}>
              Pilih File
            </Button>
          </div>

          {preview.isError && (
            <p className="text-[12px] text-danger">
              {preview.error instanceof ApiError ? preview.error.message : 'Gagal mengunggah file.'}
            </p>
          )}

          <Button type="button" onClick={handleUpload} loading={preview.isPending} disabled={!file}>
            Unggah &amp; Pratinjau
          </Button>
        </div>
      )}

      {step === 'preview' && preview.data && (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-4 gap-3">
            <StatTile label="Total" value={preview.data.summary.total} />
            <StatTile label="Baru" value={preview.data.summary.create} />
            <StatTile label="Update" value={preview.data.summary.update} />
            <StatTile label="Error" value={preview.data.summary.error} variant={preview.data.summary.error > 0 ? 'danger' : 'default'} />
          </div>

          {preview.data.rows.length === 0 ? (
            <EmptyState message="File tidak berisi baris data." />
          ) : (
            <div>
              {preview.data.rows.map((row) => (
                <ListRow
                  key={row.row}
                  title={rowDisplayName(row)}
                  subtitle={row.action === 'error' ? row.messages.join('; ') || rowSubtitle(row) : rowSubtitle(row)}
                  trailing={
                    <Tag variant={row.action === 'error' ? 'danger' : row.action === 'update' ? 'neutral' : 'now'}>
                      {ACTION_LABEL[row.action]}
                    </Tag>
                  }
                />
              ))}
            </div>
          )}

          {commit.isError && (
            <p className="text-[12px] text-danger">
              {commit.error instanceof ApiError ? commit.error.message : 'Gagal melakukan commit import.'}
            </p>
          )}

          <div className="flex flex-col gap-2">
            <Button type="button" onClick={handleCommit} loading={commit.isPending} disabled={validCount === 0}>
              Commit Import ({validCount} baris)
            </Button>
            <Button type="button" variant="secondary" onClick={() => setStep('upload')}>
              Unggah Ulang
            </Button>
          </div>
        </div>
      )}

      {step === 'result' && commit.data && (
        <div className="flex flex-col gap-6">
          <div className="grid grid-cols-3 gap-3">
            <StatTile label="Dibuat" value={commit.data.created} />
            <StatTile label="Diperbarui" value={commit.data.updated} />
            <StatTile label="Dilewati" value={commit.data.skipped} />
          </div>
          <Button type="button" onClick={() => navigate(backTo)}>
            Selesai — Lihat Daftar {label === 'siswa' ? 'Siswa' : 'Guru'}
          </Button>
        </div>
      )}
    </div>
  );
}
