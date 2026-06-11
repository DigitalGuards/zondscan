"use client"

import React from "react"

/**
 * ZondScan logo with a lightweight video animation.
 *
 * Replaces the old 5.1 MB animated GIF (1200x800, 255 frames) that was
 * eagerly preloaded on every page. The animation is now a ~170 KB VP9
 * webm (~190 KB H.264 mp4 fallback) composited over the sidebar color
 * (#1a1a1a), because H.264 cannot carry an alpha channel.
 *
 * A 12 KB transparent webp renders immediately as the base layer; the
 * video fades in only once it is actually playing, so browsers that
 * block autoplay (e.g. iOS Low Power Mode) gracefully keep the static
 * logo with no black box or play-button overlay.
 */
export default function AnimatedLogo({
  className = "",
  alt = "",
}: {
  className?: string
  /** Accessible name. Leave empty (decorative) when the surrounding link already has visible text. */
  alt?: string
}) {
  const [playing, setPlaying] = React.useState(false)

  // React renders the muted attribute but does not reliably set the DOM
  // property before the autoplay check in all browsers (React has no
  // defaultMuted prop), so set it imperatively on mount.
  const videoRef = React.useCallback((el: HTMLVideoElement | null) => {
    if (el) el.muted = true
  }, [])

  return (
    <div className={`relative h-full w-full ${className}`}>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/ZondScan_Logo_static.webp"
        alt={alt}
        className="absolute inset-0 h-full w-full object-contain"
      />
      <video
        ref={videoRef}
        muted
        autoPlay
        loop
        playsInline
        preload="auto"
        aria-hidden="true"
        tabIndex={-1}
        onPlaying={() => setPlaying(true)}
        className={`absolute inset-0 h-full w-full object-contain transition-opacity duration-300 ${
          playing ? "opacity-100" : "opacity-0"
        }`}
      >
        <source src="/ZondScan_Logo_anim.webm" type="video/webm" />
        <source src="/ZondScan_Logo_anim.mp4" type="video/mp4" />
      </video>
    </div>
  )
}
