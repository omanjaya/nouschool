import { useEffect, useState } from 'react';
import { Bell, X } from 'lucide-react';
import { Card } from '../../components/ui/Card';
import { Button } from '../../components/ui/Button';
import { useToast } from '../../components/ui/Toast';
import { enablePush, getPushState, type PushState } from '../../lib/push';

const DISMISS_KEY = 'push_prompt_dismissed';

/**
 * Banner sekali-tampil di Beranda — hanya untuk perangkat yang push-nya
 * BELUM aktif (`disabled`, bukan `unsupported`/`denied`/`enabled`). Tertutup
 * permanen (per perangkat) lewat flag localStorage begitu ditutup atau
 * berhasil diaktifkan (docs/08-notification.md).
 */
export function PushPromptBanner() {
  const [state, setState] = useState<PushState | null>(null);
  const [dismissed, setDismissed] = useState(() => localStorage.getItem(DISMISS_KEY) === '1');
  const [loading, setLoading] = useState(false);
  const { showToast } = useToast();

  useEffect(() => {
    let cancelled = false;
    getPushState()
      .then((s) => {
        if (!cancelled) setState(s);
      })
      .catch(() => {
        if (!cancelled) setState('unsupported');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (dismissed || state !== 'disabled') return null;

  function handleDismiss() {
    localStorage.setItem(DISMISS_KEY, '1');
    setDismissed(true);
  }

  async function handleEnable() {
    setLoading(true);
    try {
      await enablePush();
      localStorage.setItem(DISMISS_KEY, '1');
      setDismissed(true);
      showToast('Notifikasi push diaktifkan.');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Gagal mengaktifkan notifikasi push.', 'error');
    } finally {
      setLoading(false);
    }
  }

  return (
    <Card className="flex items-start gap-3">
      <Bell size={20} strokeWidth={2} className="mt-0.5 shrink-0 text-primary" aria-hidden="true" />
      <div className="min-w-0 flex-1">
        <p className="text-[14px] text-ink">Aktifkan notifikasi supaya tidak ketinggalan info.</p>
        <Button variant="secondary" onClick={handleEnable} loading={loading} className="mt-3">
          Aktifkan
        </Button>
      </div>
      <button
        type="button"
        onClick={handleDismiss}
        aria-label="Tutup"
        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
      >
        <X size={16} strokeWidth={2} aria-hidden="true" />
      </button>
    </Card>
  );
}
