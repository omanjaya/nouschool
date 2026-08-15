import { useMutation } from '@tanstack/react-query';
import { api } from '../../lib/api';
import type { InterestLeadInput } from '../../lib/types';

/** POST /api/public/interest — publik, hanya di host platform (landing page). */
export function useSubmitInterest() {
  return useMutation({
    mutationFn: (input: InterestLeadInput) => api.post<undefined>('/public/interest', input),
  });
}
