import { useEffect, useRef, useState } from 'react';
import { parseWallClock } from './clock';

/**
 * Jam TV yang tick per detik di client, disinkronkan ulang dari `date`+`time`
 * server tiap kali payload baru datang (polling `/api/tv/board` tiap 30 dtk,
 * lihat `syncToken` = `generated_at`). Server hanya kirim jam sampai menit,
 * jadi presisi detik baru akurat persis setelah sinkronisasi — cukup untuk
 * jam dinding TV, bukan jam presisi tinggi.
 */
export function useServerClock(date: string | undefined, time: string | undefined, syncToken: string | undefined): Date {
  const offsetRef = useRef(0);
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    if (!date || !time) return;
    const serverAt = parseWallClock(date, time);
    if (!serverAt) return;
    offsetRef.current = serverAt.getTime() - Date.now();
    setNow(new Date(Date.now() + offsetRef.current));
    // syncToken sengaja dipakai sebagai dependency murni untuk memicu resinkronisasi
    // saat payload baru datang meski date/time bertepatan (mis. detik yang sama).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [date, time, syncToken]);

  useEffect(() => {
    const id = setInterval(() => {
      setNow(new Date(Date.now() + offsetRef.current));
    }, 1000);
    return () => clearInterval(id);
  }, []);

  return now;
}
