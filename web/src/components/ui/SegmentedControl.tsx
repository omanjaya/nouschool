export interface SegmentedOption<T extends string> {
  value: T;
  label: string;
}

interface SegmentedControlProps<T extends string> {
  options: SegmentedOption<T>[];
  value: T;
  onChange: (value: T) => void;
  className?: string;
}

/** Filter pendek (mis. rentang tanggal) — docs/10-design-system.md #6. */
export function SegmentedControl<T extends string>({ options, value, onChange, className = '' }: SegmentedControlProps<T>) {
  return (
    <div role="tablist" className={`inline-flex gap-1 rounded-lg border border-line bg-surface-2 p-1 ${className}`}>
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          role="tab"
          aria-selected={opt.value === value}
          onClick={() => onChange(opt.value)}
          className={`min-h-8 rounded-lg px-3 text-[13px] font-medium transition-colors duration-150 ${
            opt.value === value ? 'bg-surface text-ink' : 'text-muted hover:text-ink'
          }`}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
