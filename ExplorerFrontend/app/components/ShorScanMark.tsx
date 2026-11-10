/**
 * ShorScan logo mark: a block omega with one amber scan row.
 *
 * Inline SVG. The light blocks follow the theme text colour and the amber
 * parts follow the theme accent, so the mark stays visible on the light,
 * dim and dark themes (the light theme accent is a darker amber).
 *
 * Usage: <ShorScanMark className="h-[38px] w-[38px]" />
 */
const LIGHT_BLOCKS: ReadonlyArray<readonly [number, number]> = [
  [16.82, 6.48],
  [27.16, 6.48],
  [37.5, 6.48],
  [6.48, 16.82],
  [47.84, 16.82],
  [16.82, 37.5],
  [37.5, 37.5],
  [6.48, 47.84],
  [16.82, 47.84],
  [37.5, 47.84],
  [47.84, 47.84],
];

const AMBER_BLOCKS: ReadonlyArray<readonly [number, number]> = [
  [6.48, 27.16],
  [47.84, 27.16],
];

export default function ShorScanMark({
  className = '',
  title,
}: {
  className?: string;
  /** Accessible name. Leave empty (decorative) when the surrounding link already has visible text. */
  title?: string;
}) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 64 64"
      className={className}
      {...(title ? { role: 'img', 'aria-label': title } : { 'aria-hidden': true })}
    >
      {LIGHT_BLOCKS.map(([x, y]) => (
        <rect key={`${x}-${y}`} x={x} y={y} width="9.68" height="9.68" rx="1.4" className="fill-text-primary" />
      ))}
      {AMBER_BLOCKS.map(([x, y]) => (
        <rect key={`${x}-${y}`} x={x} y={y} width="9.68" height="9.68" rx="1.4" className="fill-accent" />
      ))}
      <rect x="16.82" y="30.57" width="30.36" height="2.86" rx="1.2" className="fill-accent" />
    </svg>
  );
}
