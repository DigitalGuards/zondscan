"use client"

import React from "react"
import Link from 'next/link'
import Image from 'next/image'
import { useRouter } from 'next/navigation'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon } from '@heroicons/react/20/solid'
import LookUpIcon from '../../public/lookup.svg'
import TokenIcon from '../../public/token.svg'
import PartnerHandshakeIcon from '../../public/partner-handshake-icon.svg'
import BlockchainIcon from '../../public/blockchain-icon.svg'
import ContractIcon from '../../public/contract.svg'
import QRLFavicon from '../../public/favicon.ico'
import UserCircleIcon from '../../public/send.svg'

const blockchain = [
  { name: 'View Transactions', description: 'View all Transactions', href: '/transactions/1', imgSrc: PartnerHandshakeIcon },
  { name: 'View Blocks', description: 'View all Blocks', href: '/blocks/1', imgSrc: BlockchainIcon },
  { name: 'View Contracts', description: 'Explore QRL contracts', href: '/contracts', imgSrc: ContractIcon },
]

const tools = [
  { name: 'Balance Checker', description: 'Check Account balance', href: '/checker', imgSrc: LookUpIcon },
  { name: 'Unit Converter', description: 'Convert QRL currencies', href: '/converter', imgSrc: TokenIcon },
]

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function Sidebar() {
  const router = useRouter();

  const navigateTo = (href: string) => {
    router.push(href);
  };

  return (
    <aside className="fixed left-0 top-0 h-full w-64 overflow-y-auto z-50
                      bg-gradient-to-b from-[#1a1a1a] via-[#1a1a1a] to-[#1f1f1f]
                      border-r border-[#2d2d2d] shadow-[4px_0_24px_rgba(0,0,0,0.2)]">
      <div className="p-6">
        <Link href="/" className="flex items-center gap-3 mb-10 px-2 group">
          <div className="w-8 h-8 relative">
            <Image 
              src={QRLFavicon} 
              alt="QRL" 
              fill
              sizes="32px"
              style={{ objectFit: 'contain' }}
              loading="eager"
              className="group-hover:scale-110 transition-transform duration-300"
            />
          </div>
          <span className="text-lg font-semibold text-gray-300 group-hover:text-[#ffa729] transition-colors">
            Zond Explorer
          </span>
        </Link>

        <nav className="space-y-5">
          <Disclosure as="div" defaultOpen>
            {({ open }) => (
              <>
                <Disclosure.Button className="flex w-full items-center justify-between rounded-xl 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-5 py-4 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                  <span>Blockchain</span>
                  <ChevronDownIcon
                    className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                  />
                </Disclosure.Button>
                <Disclosure.Panel className="mt-3 space-y-2 pl-3">
                  {blockchain.map((item) => (
                    <button
                      key={item.name}
                      onClick={() => navigateTo(item.href)}
                      className="flex w-full items-center gap-3 px-4 py-3 text-sm text-gray-300 
                               hover:bg-[#2d2d2d] rounded-lg transition-all duration-200
                               hover:text-[#ffa729] group"
                    >
                      <div className="w-5 h-5 relative">
                        <Image
                          src={item.imgSrc}
                          alt={item.name}
                          fill
                          sizes="20px"
                          style={{ objectFit: 'contain' }}
                          className="[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                   group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                   transition-[filter]"
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
                <Disclosure.Button className="flex w-full items-center justify-between rounded-xl 
                                           bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                                           px-5 py-4 text-left text-sm font-medium 
                                           text-gray-300 hover:bg-[#3d3d3d] transition-colors
                                           shadow-md">
                  <span>Tools</span>
                  <ChevronDownIcon
                    className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 text-[#ffa729] transition-transform duration-200')}
                  />
                </Disclosure.Button>
                <Disclosure.Panel className="mt-3 space-y-2 pl-3">
                  {tools.map((item) => (
                    <button
                      key={item.name}
                      onClick={() => navigateTo(item.href)}
                      className="flex w-full items-center gap-3 px-4 py-3 text-sm text-gray-300 
                               hover:bg-[#2d2d2d] rounded-lg transition-all duration-200
                               hover:text-[#ffa729] group"
                    >
                      <div className="w-5 h-5 relative">
                        <Image
                          src={item.imgSrc}
                          alt={item.name}
                          fill
                          sizes="20px"
                          style={{ objectFit: 'contain' }}
                          className="[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                                   group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                                   transition-[filter]"
                        />
                      </div>
                      <span className="truncate">{item.name}</span>
                    </button>
                  ))}
                </Disclosure.Panel>
              </>
            )}
          </Disclosure>

          <button
            onClick={() => navigateTo('/richlist')}
            className="flex w-full items-center px-5 py-4 text-sm font-medium 
                     text-gray-300 hover:text-[#ffa729] hover:bg-[#2d2d2d] 
                     rounded-xl transition-all duration-200"
          >
            Richlist
          </button>

          <Link
            href="https://qrlwallet.com"
            target="_blank"
            rel="noopener noreferrer"
            className="flex w-full items-center gap-3 px-5 py-4 text-sm font-medium 
                     text-gray-300 hover:text-[#ffa729] hover:bg-[#2d2d2d] 
                     rounded-xl transition-all duration-200 group"
          >
            <div className="w-5 h-5 relative">
              <Image
                src={UserCircleIcon}
                alt="Wallet"
                fill
                sizes="20px"
                style={{ objectFit: 'contain' }}
                className="[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(105%)] 
                         group-hover:[filter:invert(80%)_sepia(50%)_saturate(1000%)_hue-rotate(330deg)_brightness(125%)] 
                         transition-[filter]"
              />
            </div>
            QRL Zond Web Wallet
          </Link>
        </nav>
      </div>
    </aside>
  )
}
