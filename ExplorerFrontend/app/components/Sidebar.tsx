"use client"

import React from "react"
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import AnimatedLogo from './AnimatedLogo'
import {
  Bars3Icon,
  XMarkIcon,
  QuestionMarkCircleIcon,
  ArrowTopRightOnSquareIcon,
  CodeBracketIcon,
  HomeIcon,
  ArrowsRightLeftIcon,
  ClockIcon,
  CubeIcon,
  BoltIcon,
  CalendarDaysIcon,
  ShieldCheckIcon,
  DocumentTextIcon,
  MagnifyingGlassIcon,
  BeakerIcon,
  CalculatorIcon,
  TrophyIcon,
  WalletIcon,
  CubeTransparentIcon,
} from '@heroicons/react/24/outline'

interface NavItem {
  name: string
  href: string
  icon: React.ComponentType<React.SVGProps<SVGSVGElement>>
  external?: boolean
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: 'Blockchain',
    items: [
      { name: 'Overview', href: '/', icon: HomeIcon },
      { name: 'Transactions', href: '/transactions/1', icon: ArrowsRightLeftIcon },
      { name: 'Pending', href: '/pending/1', icon: ClockIcon },
      { name: 'Blocks', href: '/blocks/1', icon: CubeIcon },
      { name: 'Gas Tracker', href: '/gas', icon: BoltIcon },
    ],
  },
  {
    label: 'Consensus',
    items: [
      { name: 'Epochs', href: '/epochs/1', icon: CalendarDaysIcon },
      { name: 'Validators', href: '/validators', icon: ShieldCheckIcon },
    ],
  },
  {
    label: 'Tools',
    items: [
      { name: 'Smart Contracts', href: '/contracts', icon: DocumentTextIcon },
      { name: 'Balance Checker', href: '/checker', icon: MagnifyingGlassIcon },
      { name: 'Testnet Faucet', href: '/faucet', icon: BeakerIcon },
      { name: 'Unit Converter', href: '/converter', icon: CalculatorIcon },
      { name: 'Richlist', href: '/richlist', icon: TrophyIcon },
    ],
  },
  {
    label: 'Ecosystem',
    items: [
      { name: 'MyQRLWallet', href: 'https://myqrlwallet.com', icon: WalletIcon, external: true },
      { name: 'dApp Example', href: '/dapp-example/', icon: CubeTransparentIcon },
      { name: 'API Explorer', href: '/api-explorer', icon: CodeBracketIcon },
      { name: 'FAQ', href: '/faq', icon: QuestionMarkCircleIcon },
    ],
  },
]

function isActive(pathname: string, href: string): boolean {
  // Exact match
  if (pathname === href) return true
  // Match base path for paginated routes like /transactions/1, /blocks/1, /pending/1
  const basePath = href.replace(/\/\d+$/, '')
  if (basePath !== href && pathname.startsWith(basePath + '/')) return true
  // Match parent paths for detail pages (e.g., /block/123 matches /blocks/1)
  if (href.startsWith('/blocks') && pathname.startsWith('/block/')) return true
  if (href.startsWith('/epochs') && pathname.startsWith('/epochs/')) return true
  if (href.startsWith('/transactions') && pathname.startsWith('/tx/')) return true
  if (href === '/contracts' && pathname.startsWith('/contracts')) return true
  return false
}

function NavLink({ item, pathname }: { item: NavItem; pathname: string }): JSX.Element {
  const active = !item.external && isActive(pathname, item.href)
  const Icon = item.icon
  const classes = `relative flex w-full items-center gap-2.5 px-3 py-2 text-sm rounded-lg
                   transition-colors duration-200 group whitespace-nowrap
                   ${active
                     ? 'bg-accent/10 text-accent font-medium'
                     : 'text-text-secondary hover:bg-surface-2 hover:text-text-primary'
                   }`

  const inner = (
    <>
      {/* Active glow bar: overlaid, so selection never shifts the label */}
      <span
        aria-hidden="true"
        className={`absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-accent
                    shadow-glow-accent transition-opacity duration-200
                    ${active ? 'opacity-100' : 'opacity-0'}`}
      />
      <Icon
        className={`w-[18px] h-[18px] flex-shrink-0 transition-colors
                    ${active ? 'text-accent' : 'text-text-muted group-hover:text-accent'}`}
        aria-hidden="true"
      />
      <span className="truncate">{item.name}</span>
      {item.external && (
        <ArrowTopRightOnSquareIcon
          className="w-3.5 h-3.5 text-text-muted ml-auto flex-shrink-0"
          aria-hidden="true"
        />
      )}
    </>
  )

  if (item.external) {
    return (
      <a href={item.href} target="_blank" rel="noopener noreferrer" className={classes}>
        {inner}
      </a>
    )
  }

  // The dApp example is a static bundle under public/, not an App Router
  // route: it needs a hard navigation via <a>, not <Link>.
  if (item.href === '/dapp-example/') {
    return (
      <a href={item.href} className={classes}>
        {inner}
      </a>
    )
  }

  return (
    <Link href={item.href} aria-current={active ? 'page' : undefined} className={classes}>
      {inner}
    </Link>
  )
}

export default function Sidebar(): JSX.Element {
  const pathname = usePathname()
  const [isOpen, setIsOpen] = React.useState(false)
  const [isVisible, setIsVisible] = React.useState(true)
  const lastScrollY = React.useRef(0)

  // Lock body scroll when menu is open
  React.useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = 'unset'
    }
    return () => {
      document.body.style.overflow = 'unset'
    }
  }, [isOpen])

  // Handle scroll behavior for mobile top bar
  React.useEffect(() => {
    const handleScroll = (): void => {
      const currentScrollY = window.scrollY
      if (currentScrollY < 10) {
        setIsVisible(true)
      } else if (currentScrollY > lastScrollY.current) {
        setIsVisible(false)
      } else {
        setIsVisible(true)
      }
      lastScrollY.current = currentScrollY
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [])

  // Close sidebar on Escape
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent): void => {
      if (e.key === 'Escape' && isOpen) {
        setIsOpen(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen])

  // Close sidebar on route change (mobile). React's "adjusting state on
  // prop change" pattern: detect mid-render and reset without queuing a
  // post-render setState in an effect (set-state-in-effect rule).
  const [prevPath, setPrevPath] = React.useState(pathname)
  if (prevPath !== pathname) {
    setPrevPath(pathname)
    if (isOpen) setIsOpen(false)
  }

  return (
    <>
      {/* Mobile top bar: h-14 everywhere so the drawer (top-14) and the
          content offset (mt-14 in layout.tsx) line up exactly. */}
      <div className={`lg:hidden fixed top-0 left-0 right-0 z-50 h-14
                       bg-background/80 backdrop-blur-xl border-b border-border
                       transition-transform duration-300 ${
        isVisible ? 'translate-y-0' : '-translate-y-full'
      }`}>
        <div className="flex h-full items-center justify-center px-4 relative">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="absolute left-4 p-2 rounded-lg bg-surface-2 border border-border
                       text-text-secondary hover:text-text-primary hover:bg-surface-3
                       transition-colors"
            aria-label="Toggle menu"
            aria-expanded={isOpen}
          >
            {isOpen ? (
              <XMarkIcon className="h-5 w-5" />
            ) : (
              <Bars3Icon className="h-5 w-5" />
            )}
          </button>
          <Link href="/" className="group">
            <div className="relative w-12 h-10">
              <AnimatedLogo alt="ZondScan home" />
            </div>
          </Link>
        </div>
      </div>

      {/* Mobile backdrop */}
      {isOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-black/60 backdrop-blur-sm z-40"
          onClick={() => setIsOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Sidebar rail: translucent over the page atmosphere */}
      <aside
        className={`fixed left-0 bottom-0 overflow-y-auto overscroll-contain z-50
                    bg-background lg:bg-background/75 lg:backdrop-blur-xl border-r border-border
                    transition-transform duration-300 ease-in-out
                    ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
                    w-64 lg:top-0 top-14`}
        aria-label="Main navigation"
      >
        <div className="px-4 pt-6 pb-8">
          <Link href="/" className="flex flex-col items-center mb-7 px-1 group">
            <div className="w-28 h-20 relative">
              <AnimatedLogo />
            </div>
            <div className="flex flex-col items-center mt-3 gap-1.5">
              <span className="font-display text-xl font-semibold tracking-wide text-text-primary
                               group-hover:text-accent transition-colors">
                ZondScan
              </span>
              <span className="eyebrow">QRL 2.0 Explorer</span>
            </div>
          </Link>

          <nav className="space-y-6">
            {navGroups.map((group) => (
              <div key={group.label}>
                <p className="eyebrow px-3 mb-2">{group.label}</p>
                <div className="space-y-0.5">
                  {group.items.map((item) => (
                    <NavLink key={item.name} item={item} pathname={pathname} />
                  ))}
                </div>
              </div>
            ))}
          </nav>
        </div>
      </aside>
    </>
  )
}
