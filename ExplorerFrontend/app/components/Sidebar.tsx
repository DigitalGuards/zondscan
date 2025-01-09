"use client"

import React from "react"
import Link from 'next/link'
import Image from 'next/image'
import { useRouter } from 'next/navigation'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon, Bars3Icon, XMarkIcon } from '@heroicons/react/20/solid'
import LookUpIcon from '../../public/lookup.svg'
import TokenIcon from '../../public/token.svg'
import PartnerHandshakeIcon from '../../public/partner-handshake-icon.svg'
import BlockchainIcon from '../../public/blockchain-icon.svg'
import ContractIcon from '../../public/contract.svg'
import QRLFavicon from '../../public/favicon.ico'
import SendIcon from '../../public/send.svg'
import RichIcon from '../../public/favis/favicon-32x32.png'

const blockchain = [
  { name: 'Latest Transactions', description: 'View all Transactions', href: '/transactions/1', imgSrc: PartnerHandshakeIcon },
  { name: 'Pending Transactions', description: 'View pending transactions', href: '/pending/1', imgSrc: PartnerHandshakeIcon },
  { name: 'Latest Blocks', description: 'View all Blocks', href: '/blocks/1', imgSrc: BlockchainIcon },
  { name: 'Validators', description: 'Network Validators', href: '/validators', imgSrc: ContractIcon },
]

const smartContracts = [
  { name: 'Smart Contracts', description: 'Explore QRL contracts', href: '/contracts', imgSrc: ContractIcon },
  //{ name: 'Tokens', description: 'View QRL tokens', href: '/tokens', imgSrc: TokenIcon },
  // NFTs section commented out until implementation
  // { name: 'NFTs', description: 'View QRL NFTs', href: '/nfts', imgSrc: TokenIcon },
]

const tools = [
  { name: 'Balance Checker', description: 'Check Account balance', href: '/checker', imgSrc: LookUpIcon },
  { name: 'Unit Converter', description: 'Convert QRL currencies', href: '/converter', imgSrc: TokenIcon },
  { name: 'Richlist', description: 'Top QRL holders', href: '/richlist', imgSrc: RichIcon },
]

const ValidatorIcon = () => (
  <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
  </svg>
);

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function Sidebar() {
  const router = useRouter();
  const [isOpen, setIsOpen] = React.useState(false);
  const [isVisible, setIsVisible] = React.useState(true);
  const [lastScrollY, setLastScrollY] = React.useState(0);

  // Lock body scroll when menu is open
  React.useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }
    return () => {
      document.body.style.overflow = 'unset';
    };
  }, [isOpen]);

  // Handle scroll behavior
  React.useEffect(() => {
    const handleScroll = () => {
      const currentScrollY = window.scrollY;
      
      if (currentScrollY > lastScrollY) {
        setIsVisible(false); // Scrolling down
      } else {
        setIsVisible(true);  // Scrolling up
      }
      
      setLastScrollY(currentScrollY);
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, [lastScrollY]);

  const navigateTo = (href: string) => {
    router.push(href);
    setIsOpen(false);
  };

  return (
    <>
      {/* Mobile menu button */}
      <div className={`lg:hidden fixed top-0 left-0 right-0 z-50 bg-[#1a1a1a] transition-transform duration-300 ${
        isVisible ? 'translate-y-0' : '-translate-y-full'
      }`}>
        <div className="flex items-center justify-between px-4 py-3 border-b border-[#2d2d2d]">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="p-2 rounded-lg bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d] transition-colors"
            aria-label="Toggle menu"
          >
            {isOpen ? (
              <XMarkIcon className="h-6 w-6" />
            ) : (
              <Bars3Icon className="h-6 w-6" />
            )}
          </button>
          <span className="text-lg font-semibold text-gray-300">ZondScan Explorer</span>
          <div className="relative w-8 h-8">
            <Image 
              src={QRLFavicon} 
              alt="QRL"
              fill
              sizes="32px"
              style={{ objectFit: 'contain' }}
              loading="eager"
              className="hover:scale-110 transition-transform duration-300"
            />
          </div>
        </div>
      </div>

      {/* Mobile backdrop */}
      {isOpen && (
        <div 
          className="lg:hidden fixed inset-0 bg-black bg-opacity-50 z-40"
          onClick={() => setIsOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside className={`fixed left-0 h-full overflow-y-auto z-50
                      bg-gradient-to-b from-[#1a1a1a] via-[#1a1a1a] to-[#1f1f1f]
                      border-r border-[#2d2d2d] shadow-[4px_0_24px_rgba(0,0,0,0.2)]
                      transition-all duration-300 ease-in-out
                      ${isOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
                      w-56 lg:top-0 top-[53px]`}>
        <div className="p-4">
          <Link href="/" className="flex items-center gap-1.5 mb-6 px-1 group" onClick={() => setIsOpen(false)}>
            <div className="w-6 h-6 relative">
              <Image 
                src={QRLFavicon} 
                alt="QRL" 
                fill
                sizes="24px"
                style={{ objectFit: 'contain' }}
                loading="eager"
                className="group-hover:scale-110 transition-transform duration-300"
              />
            </div>
            <span className="text-base font-medium text-gray-300 whitespace-nowrap group-hover:text-[#ffa729] transition-colors">
              ZondScan Explorer
            </span>
          </Link>

          <nav className="space-y-3">
            <Disclosure as="div" defaultOpen>
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between rounded-lg 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-4 py-2.5 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                    <span className="text-base">Blockchain</span>
                    <ChevronDownIcon
                      className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="mt-2 space-y-1">
                    {blockchain.map((item) => (
                      <button
                        key={item.name}
                        onClick={() => navigateTo(item.href)}
                        className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300 
                               hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                               hover:text-[#ffa729] group"
                      >
                        <div className="w-5 h-5 relative">
                          <Image
                            src={item.imgSrc}
                            alt={item.name}
                            fill
                            sizes="20px"
                            style={{ objectFit: 'contain' }}
                            className={item.name === 'Richlist' ? 'transition-[filter]' : `[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                   group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                   transition-[filter]`}
                          />
                        </div>
                        <span className="truncate">{item.name}</span>
                      </button>
                    ))}
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>

            <Disclosure as="div" defaultOpen>
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between rounded-lg 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-4 py-2.5 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                    <span className="text-base">Smart Contracts</span>
                    <ChevronDownIcon
                      className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="mt-2 space-y-1">
                    {smartContracts.map((item) => (
                      <button
                        key={item.name}
                        onClick={() => navigateTo(item.href)}
                        className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300 
                               hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                               hover:text-[#ffa729] group"
                      >
                        <div className="w-5 h-5 relative">
                          <Image
                            src={item.imgSrc}
                            alt={item.name}
                            fill
                            sizes="20px"
                            style={{ objectFit: 'contain' }}
                            className={item.name === 'Richlist' ? 'transition-[filter]' : `[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                   group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                   transition-[filter]`}
                          />
                        </div>
                        <span className="truncate">{item.name}</span>
                      </button>
                    ))}
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>

            <Disclosure as="div" defaultOpen>
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between rounded-lg 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-4 py-2.5 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                    <span className="text-base">Tools</span>
                    <ChevronDownIcon
                      className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="mt-2 space-y-1">
                    {tools.map((item) => (
                      <button
                        key={item.name}
                        onClick={() => navigateTo(item.href)}
                        className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300 
                               hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                               hover:text-[#ffa729] group"
                      >
                        <div className="w-5 h-5 relative">
                          <Image
                            src={item.imgSrc}
                            alt={item.name}
                            fill
                            sizes="20px"
                            style={{ objectFit: 'contain' }}
                            className={item.name === 'Richlist' ? 'transition-[filter]' : `[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                   group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                   transition-[filter]`}
                          />
                        </div>
                        <span className="truncate">{item.name}</span>
                      </button>
                    ))}
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>

            <Disclosure as="div" defaultOpen>
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between rounded-lg 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-4 py-2.5 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                    <span className="text-base">Wallet & FAQ</span>
                    <ChevronDownIcon
                      className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="mt-2 space-y-1">
                    <a
                      href="https://qrlwallet.com"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300 
                             hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                             hover:text-[#ffa729] group"
                    >
                      <div className="w-5 h-5 relative">
                        <Image
                          src={SendIcon}
                          alt="Wallet"
                          fill
                          sizes="20px"
                          style={{ objectFit: 'contain' }}
                          className="[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                 group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                 transition-[filter]"
                        />
                      </div>
                      <span className="truncate">QRL Zond Wallet</span>
                    </a>
                    <button
                      onClick={() => navigateTo('/faq')}
                      className="flex w-full items-center gap-2 px-3 py-2.5 text-sm text-gray-300 
                             hover:bg-[#2d2d2d] rounded-md transition-all duration-200
                             hover:text-[#ffa729] group"
                    >
                      <div className="w-5 h-5 relative">
                        <Image
                          src={SendIcon}
                          alt="FAQ"
                          fill
                          sizes="20px"
                          style={{ objectFit: 'contain' }}
                          className="[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                 group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                 transition-[filter]"
                        />
                      </div>
                      <span className="truncate">FAQ</span>
                    </button>
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>
          </nav>
        </div>
      </aside>
    </>
  )
}
