"use client"

import React from "react"
import Link from 'next/link'
import Image from 'next/image'
import { usePathname } from 'next/navigation'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon, Bars3Icon, XMarkIcon, QuestionMarkCircleIcon, ArrowTopRightOnSquareIcon } from '@heroicons/react/20/solid'
import LookUpIcon from '../../public/lookup.svg'
import TokenIcon from '../../public/token.svg'
import PartnerHandshakeIcon from '../../public/partner-handshake-icon.svg'
import BlockchainIcon from '../../public/blockchain-icon.svg'
import ContractIcon from '../../public/contract.svg'
import SendIcon from '../../public/send.svg'
import RichIcon from '../../public/favis/favicon-32x32.png'

interface NavItem {
  name: string
  description: string
  href: string
  imgSrc: typeof PartnerHandshakeIcon
}

interface NavGroup {
  label: string
  items: NavItem[]
}

const navGroups: NavGroup[] = [
  {
    label: 'Blockchain',
    items: [
      { name: 'Latest Transactions', description: 'View all Transactions', href: '/transactions/1', imgSrc: PartnerHandshakeIcon },
      { name: 'Pending Transactions', description: 'View pending transactions', href: '/pending/1', imgSrc: PartnerHandshakeIcon },
      { name: 'Latest Blocks', description: 'View all Blocks', href: '/blocks/1', imgSrc: BlockchainIcon },
      { name: 'Validators', description: 'Network Validators', href: '/validators', imgSrc: ContractIcon },
    ],
  },
  {
    label: 'Tools',
    items: [
      { name: 'Smart Contracts', description: 'View QRL contracts', href: '/contracts', imgSrc: ContractIcon },
      { name: 'Balance Checker', description: 'Check Account balance', href: '/checker', imgSrc: LookUpIcon },
      { name: 'Unit Converter', description: 'Convert QRL currencies', href: '/converter', imgSrc: TokenIcon },
    ],
  },
]

// Flat links (single items that don't need accordion groups)
const flatLinks: NavItem[] = [
  { name: 'Richlist', description: 'Top QRL holders', href: '/richlist', imgSrc: RichIcon },
]

const ICON_FILTER = "[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)]"
const ICON_FILTER_HOVER = "group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)]"
const ICON_FILTER_ACTIVE = "[filter:invert(70%)_sepia(80%)_saturate(2000%)_hue-rotate(345deg)_brightness(110%)]"

function isActive(pathname: string, href: string): boolean {
  // Exact match
  if (pathname === href) return true
  // Match base path for paginated routes like /transactions/1, /blocks/1, /pending/1
  const basePath = href.replace(/\/\d+$/, '')
  if (basePath !== href && pathname.startsWith(basePath + '/')) return true
  // Match parent paths for detail pages (e.g., /block/123 matches /blocks/1)
  if (href.startsWith('/blocks') && pathname.startsWith('/block/')) return true
  if (href.startsWith('/transactions') && pathname.startsWith('/tx/')) return true
  if (href === '/contracts' && pathname.startsWith('/contracts')) return true
  return false
}

function getGroupForPath(pathname: string): string | null {
  for (const group of navGroups) {
    for (const item of group.items) {
      if (isActive(pathname, item.href)) return group.label
    }
  }
  return null
}

export default function Sidebar(): JSX.Element {
  const pathname = usePathname()
  const [isOpen, setIsOpen] = React.useState(false)
  const [isVisible, setIsVisible] = React.useState(true)
  const lastScrollY = React.useRef(0)

  const activeGroup = getGroupForPath(pathname)

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

  // Close sidebar on route change (mobile)
  React.useEffect(() => {
    setIsOpen(false)
  }, [pathname])

  return (
    <>
      {/* Mobile menu button */}
      <div className={`lg:hidden fixed top-0 left-0 right-0 z-50 bg-[#1a1a1a] transition-transform duration-300 ${
        isVisible ? 'translate-y-0' : '-translate-y-full'
      }`}>
        <div className="flex items-center justify-center px-4 py-2 border-b border-[#2d2d2d] relative">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="absolute left-4 p-2 rounded-lg bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d] transition-colors"
            aria-label="Toggle menu"
            aria-expanded={isOpen}
          >
            {isOpen ? (
              <XMarkIcon className="h-6 w-6" />
            ) : (
              <Bars3Icon className="h-6 w-6" />
            )}
          </button>
          <Link href="/" >
            <div className="relative w-14 h-12">
              <Image
                src="/ZondScan_Logo_Z.gif"
                alt="ZondScan home"
                fill
                sizes="56px"
                style={{ objectFit: 'contain' }}
                loading="eager"
                unoptimized
                className="hover:scale-110 transition-transform duration-300"
              />
            </div>
          </Link>
        </div>
      </div>

      {/* Mobile backdrop */}
      {isOpen && (
        <div
          className="lg:hidden fixed inset-0 bg-black bg-opacity-50 z-40"
          onClick={() => setIsOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed left-0 h-full overflow-y-auto z-50
                    bg-gradient-to-b from-[#1a1a1a] via-[#1a1a1a] to-[#1f1f1f]
                    border-r border-[#2d2d2d] shadow-[4px_0_24px_rgba(0,0,0,0.2)]
                    transition-all duration-300 ease-in-out
                    ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
                    w-64 lg:top-0 top-[53px]`}
        aria-label="Main navigation"
      >
        <div className="p-4">
          <Link href="/" className="flex flex-col items-center mb-6 px-1 group">
            <div className="w-32 h-24 relative">
              <Image
                src="/ZondScan_Logo_Z.gif"
                alt="ZondScan home"
                fill
                sizes="128px"
                style={{ objectFit: 'contain' }}
                loading="eager"
                unoptimized
                className="group-hover:scale-110 transition-transform duration-300"
              />
            </div>
            <div className="flex flex-col items-center mt-2">
              <span className="text-lg font-semibold text-gray-300 whitespace-nowrap group-hover:text-[#ffa729] transition-colors">
                ZondScan
              </span>
            </div>
          </Link>

          <nav className="space-y-3">
            {/* Accordion groups */}
            {navGroups.map((group) => (
              <Disclosure key={group.label} as="div" defaultOpen={activeGroup === group.label || activeGroup === null}>
                {({ open }) => (
                  <>
                    <Disclosure.Button className="flex w-full items-center justify-between rounded-lg
                                                 bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                                 px-4 py-2.5 text-left text-sm font-medium
                                                 text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                                 shadow-md">
                      <span className="text-base">{group.label}</span>
                      <ChevronDownIcon
                        className={`${open ? 'rotate-180' : ''} h-5 w-5 text-[#ffa729] transition-transform duration-200`}
                      />
                    </Disclosure.Button>
                    <Disclosure.Panel className="mt-2 space-y-1">
                      {group.items.map((item) => {
                        const active = isActive(pathname, item.href)
                        return (
                          <Link
                            key={item.name}
                            href={item.href}
                            aria-current={active ? 'page' : undefined}
                            className={`flex w-full items-center gap-2 px-3 py-2.5 text-sm rounded-md
                                       transition-all duration-200 group whitespace-nowrap
                                       ${active
                                         ? 'bg-[#ffa729]/10 text-[#ffa729] border-l-2 border-[#ffa729]'
                                         : 'text-gray-300 hover:bg-[#2d2d2d] hover:text-[#ffa729]'
                                       }`}
                          >
                            <div className="w-5 h-5 relative flex-shrink-0">
                              <Image
                                src={item.imgSrc}
                                alt=""
                                fill
                                sizes="20px"
                                style={{ objectFit: 'contain' }}
                                className={`transition-[filter] ${
                                  item.name === 'Richlist'
                                    ? ''
                                    : active
                                      ? ICON_FILTER_ACTIVE
                                      : `${ICON_FILTER} ${ICON_FILTER_HOVER}`
                                }`}
                              />
                            </div>
                            <span className="truncate">{item.name}</span>
                          </Link>
                        )
                      })}
                    </Disclosure.Panel>
                  </>
                )}
              </Disclosure>
            ))}

            {/* Flat links (single items, no accordion needed) */}
            {flatLinks.map((item) => {
              const active = isActive(pathname, item.href)
              return (
                <Link
                  key={item.name}
                  href={item.href}
                  aria-current={active ? 'page' : undefined}
                  className={`flex w-full items-center gap-2 px-3 py-2.5 text-sm rounded-md
                             transition-all duration-200 group whitespace-nowrap
                             ${active
                               ? 'bg-[#ffa729]/10 text-[#ffa729] border-l-2 border-[#ffa729]'
                               : 'text-gray-300 hover:bg-[#2d2d2d] hover:text-[#ffa729]'
                             }`}
                >
                  <div className="w-5 h-5 relative flex-shrink-0">
                    <Image
                      src={item.imgSrc}
                      alt=""
                      fill
                      sizes="20px"
                      style={{ objectFit: 'contain' }}
                      className={`transition-[filter] ${
                        item.name === 'Richlist'
                          ? ''
                          : active
                            ? ICON_FILTER_ACTIVE
                            : `${ICON_FILTER} ${ICON_FILTER_HOVER}`
                      }`}
                    />
                  </div>
                  <span className="truncate">{item.name}</span>
                </Link>
              )
            })}

            {/* Separator */}
            <div className="border-t border-[#2d2d2d] my-3" />

            {/* External links & FAQ */}
            <a
              href="https://qrlwallet.com"
              target="_blank"
              rel="noopener noreferrer"
              className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300
                         hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                         hover:text-[#ffa729] group whitespace-nowrap"
            >
              <div className="w-5 h-5 relative flex-shrink-0">
                <Image
                  src={SendIcon}
                  alt=""
                  fill
                  sizes="20px"
                  style={{ objectFit: 'contain' }}
                  className={`${ICON_FILTER} ${ICON_FILTER_HOVER} transition-[filter]`}
                />
              </div>
              <span className="truncate">QRL Zond Wallet</span>
              <ArrowTopRightOnSquareIcon className="w-3.5 h-3.5 text-gray-500 ml-auto flex-shrink-0" />
            </a>
            <Link
              href="/faq"
              aria-current={pathname === '/faq' ? 'page' : undefined}
              className={`flex w-full items-center gap-2 px-3 py-2.5 text-sm rounded-md
                         transition-all duration-200 group whitespace-nowrap
                         ${pathname === '/faq'
                           ? 'bg-[#ffa729]/10 text-[#ffa729] border-l-2 border-[#ffa729]'
                           : 'text-gray-300 hover:bg-[#2d2d2d] hover:text-[#ffa729]'
                         }`}
            >
              <QuestionMarkCircleIcon className={`w-5 h-5 flex-shrink-0 ${
                pathname === '/faq' ? 'text-[#ffa729]' : 'text-gray-500 group-hover:text-[#ffa729]'
              } transition-colors`} />
              <span className="truncate">FAQ</span>
            </Link>
          </nav>
        </div>
      </aside>
    </>
  )
}
