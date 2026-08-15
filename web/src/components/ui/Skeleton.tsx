interface SkeletonProps {
  className?: string;
}

/** Placeholder loading list/kartu — bukan spinner fullscreen. */
export function Skeleton({ className = '' }: SkeletonProps) {
  return (
    <div
      aria-hidden="true"
      className={`animate-pulse rounded-lg bg-surface-2 motion-reduce:animate-none ${className}`}
    />
  );
}
