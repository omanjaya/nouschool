import { Check, Clock } from 'lucide-react';
import { Tag } from '../../components/ui/Tag';
import { formatDateTime } from '../../lib/date';
import { LATE_ARRIVAL_STAGE_LABEL, LATE_ARRIVAL_STAGE_ORDER, currentLateArrivalStage, type LateArrivalStageField } from './format';
import type { LateArrivalRecord } from '../../lib/types';

function Step({ field, record, isLast }: { field: LateArrivalStageField; record: LateArrivalRecord; isLast: boolean }) {
  const info = record[field];
  const isCurrent = !info && currentLateArrivalStage(record) === field;

  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <span
          className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-full border ${
            info ? 'border-st-hadir-line text-st-hadir' : 'border-line text-muted'
          }`}
        >
          {info ? <Check size={16} strokeWidth={2} aria-hidden="true" /> : <Clock size={16} strokeWidth={2} aria-hidden="true" />}
        </span>
        {!isLast && <span className="w-px flex-1 min-h-6 bg-line" aria-hidden="true" />}
      </div>
      <div className={isLast ? 'flex-1' : 'flex-1 pb-4'}>
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[14px] font-medium text-ink">{LATE_ARRIVAL_STAGE_LABEL[field]}</p>
          {isCurrent && <Tag variant="now">Sedang diproses</Tag>}
        </div>
        {info ? (
          <p className="text-[12px] text-muted">
            {info.by_name} · {formatDateTime(info.at)}
          </p>
        ) : !isCurrent ? (
          <p className="text-[12px] text-muted">Menunggu tahap sebelumnya</p>
        ) : null}
      </div>
    </div>
  );
}

/** Stepper 3 tahap rantai lapor terlambat: Piket → Pimpinan → Guru Kelas (docs/12-sion-parity.md Gelombang B alur 3). */
export function LateArrivalStepper({ record }: { record: LateArrivalRecord }) {
  return (
    <div className="flex flex-col">
      {LATE_ARRIVAL_STAGE_ORDER.map((field, i) => (
        <Step key={field} field={field} record={record} isLast={i === LATE_ARRIVAL_STAGE_ORDER.length - 1} />
      ))}
    </div>
  );
}
