'use client';

import { useState } from 'react';
import Image from 'next/image';
import type { ReactNode } from 'react';

/**
 * Image renderer that swaps in a caller-supplied fallback element when the
 * source URL fails to load. Used for off-chain NFT metadata images, which
 * routinely break: IPFS gateways go down, CIDs get unpinned, contracts
 * point at HTTP URLs that 404. Without a fallback the user sees the
 * browser's broken-image glyph; with this they see the existing monogram
 * stand-in.
 *
 * Renders next/image with `unoptimized` because the upstream sources are
 * arbitrary IPFS gateways / HTTP CDNs, not next.config.js-allowlisted
 * domains. The optimisation pipeline would just proxy + re-cache them
 * anyway; we skip the indirection.
 *
 * The fallback is rendered as a peer to the Image (with the Image hidden)
 * when load fails, so the caller can supply whatever shape they like
 * (monogram, gradient, tokenID slice, etc.).
 */
export interface ImageWithFallbackProps {
  src: string;
  alt: string;
  /** Element rendered when the image errors or src is empty. */
  fallback: ReactNode;
  fill?: boolean;
  sizes?: string;
  className?: string;
  width?: number;
  height?: number;
  /** Optional outer wrapper className when using `fill`. */
  wrapperClassName?: string;
}

export default function ImageWithFallback({
  src,
  alt,
  fallback,
  fill,
  sizes,
  className,
  width,
  height,
}: ImageWithFallbackProps): JSX.Element {
  const [errored, setErrored] = useState(false);
  const trimmed = (src || '').trim();
  if (!trimmed || errored) {
    return <>{fallback}</>;
  }
  return (
    <Image
      src={trimmed}
      alt={alt}
      fill={fill}
      sizes={sizes}
      width={width}
      height={height}
      className={className}
      unoptimized
      onError={() => setErrored(true)}
    />
  );
}
