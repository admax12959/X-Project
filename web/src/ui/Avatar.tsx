// A user's picture: the uploaded avatar (or the directory photo) when there is one, otherwise the
// first letter of the display name on a light tile. Avatars are served only to logged-in users,
// so a plain <img> with the session cookie is enough.
import { useEffect, useState } from 'react';
import { initialOf } from './avatarInitial.ts';

type Size = 'sm' | 'md' | 'xl';
const SIZE: Record<Size, string> = { sm: 'size-5 text-[11px]', md: 'size-6 text-xs', xl: 'size-24 text-4xl' };

export function Avatar({ name, url, size = 'md', className = '' }: { name: string; url: string | null | undefined; size?: Size; className?: string }) {
  const [broken, setBroken] = useState(false);
  // A new URL is a new picture: try it again.
  useEffect(() => setBroken(false), [url]);
  const box = `inline-block flex-none select-none overflow-hidden rounded-full align-middle ${SIZE[size]} ${className}`;
  if (url && !broken) {
    return <img src={url} alt="" aria-hidden="true" width={256} height={256} onError={() => setBroken(true)} className={`${box} border border-line object-cover`} />;
  }
  return (
    <span aria-hidden="true" className={`${box} inline-flex items-center justify-center border border-line bg-subtle font-semibold text-text-2`}>
      {initialOf(name)}
    </span>
  );
}
