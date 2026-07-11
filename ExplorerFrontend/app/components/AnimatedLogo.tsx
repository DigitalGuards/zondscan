"use client"

import React from "react"
import Image from "next/image"

/**
 * ZondScan logo mark.
 *
 * The old video animation (webm/mp4) was composited over the legacy
 * #1a1a1a sidebar color because H.264 cannot carry an alpha channel;
 * on the current deep blue-black canvas it would render as a visible
 * gray box. We keep the 12 KB transparent webp and express motion with
 * a CSS amber glow that breathes behind the mark instead, which also
 * drops ~360 KB of media from every first paint.
 */
export default function AnimatedLogo({
  className = "",
  alt = "",
}: {
  className?: string
  /** Accessible name. Leave empty (decorative) when the surrounding link already has visible text. */
  alt?: string
}) {
  return (
    <div className={`relative h-full w-full ${className}`}>
      {/* Breathing glow field behind the mark */}
      <div
        aria-hidden="true"
        className="absolute inset-[12%] rounded-full bg-accent/25 blur-2xl
                   animate-pulse [animation-duration:4s]
                   motion-reduce:animate-none"
      />
      <Image
        src="/ZondScan_Logo_static.webp"
        alt={alt}
        fill
        priority
        sizes="(max-width: 768px) 50vw, 200px"
        className="object-contain relative
                   drop-shadow-[0_0_14px_rgba(255,167,41,0.25)]
                   transition-[filter] duration-300
                   group-hover:drop-shadow-[0_0_22px_rgba(255,167,41,0.45)]"
      />
    </div>
  )
}
