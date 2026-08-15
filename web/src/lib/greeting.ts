/** Sapaan Beranda berdasar jam perangkat — dipakai `BerandaPage` & `KepsekHomePage`. */
export function getGreeting(hour: number): string {
  if (hour < 11) return 'Selamat pagi';
  if (hour < 15) return 'Selamat siang';
  if (hour < 18) return 'Selamat sore';
  return 'Selamat malam';
}
