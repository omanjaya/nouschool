import type { ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { useMe } from './api';
import { Skeleton } from '../../components/ui/Skeleton';

interface RequireLoginProps {
  children: ReactNode;
}

/** Menjaga rute ber-login: me loading → Skeleton halaman; 401 → redirect /login. */
export function RequireLogin({ children }: RequireLoginProps) {
  const { data: me, isLoading, isError } = useMe();

  if (isLoading) {
    return (
      <div className="mx-auto flex max-w-[640px] flex-col gap-4 px-5 py-6">
        <Skeleton className="h-6 w-40" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (isError || !me) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}
