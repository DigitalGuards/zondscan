"use client"

import React from "react"
import { Fragment, useState } from 'react'
import { Dialog, Disclosure, Popover, Transition } from '@headlessui/react'
import {
  ArrowPathIcon,
  Bars3Icon,
  CursorArrowRaysIcon,
  SquaresPlusIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline'
import { ChevronDownIcon, PhoneIcon, PlayCircleIcon } from '@heroicons/react/20/solid'
import AuthProfileIcon from "./AuthProfileIcon"
import Link from 'next/link'
import Image from 'next/image'
import LookUpIcon from '../../public/lookup.svg'
import TokenIcon from '../../public/token.svg'
import PartnerHandshakeIcon from '../../public/partner-handshake-icon.svg'
import LoadingIcon from '../../public/loading.svg'
import BlockchainIcon from '../../public/blockchain-icon.svg'
import ContractIcon from '../../public/contract.svg'
import QRLFavicon from '../../public/favicon.ico'

const blockchain = [
  { name: 'View Transactions', description: 'Here you can view all the Transactions', href: '/transactions/1', icon: SquaresPlusIcon, imgSrc: PartnerHandshakeIcon },
  { name: 'View Blocks', description: 'Here you can view all the Blocks', href: '/blocks/1', imgSrc: BlockchainIcon },
  { name: 'View Contracts', description: 'Explore all QRL contracts', href: '/contracts', imgSrc: ContractIcon },
]

const tools = [
  { name: 'Account Balance Checker', description: 'Check your Account balance on a certain date or block', href: '/checker', icon: null, imgSrc: LookUpIcon },
  { name: 'Unit Converter', description: 'Convert QRL <-> Any currency', href: '/converter', icon: ArrowPathIcon, imgSrc: null },
]

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function Header() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [IsDropDownBlockchain, setIsDropDownBlockchain] = useState(false);
  const [isDropDownTools, setisDropDownTools] = useState(false);

  return (
    <>
      <header className="bg-[#1a1a1a] border-b border-[#2d2d2d]">
        <nav className="mx-auto flex max-w-7xl items-center justify-between p-6 lg:px-8" aria-label="Global">
          <div className="flex lg:flex-1">
            <Link href="/" className="-m-1.5 p-1.5">
              <span className="sr-only">Quanta Explorer</span>
              <Image className="h-8 w-auto" src={QRLFavicon} alt="QRL Favicon" />
            </Link>
          </div>
          <div className="flex lg:hidden">
            <button
              type="button"
              className="-m-2.5 inline-flex items-center justify-center rounded-md p-2.5 text-gray-300"
              onClick={() => setMobileMenuOpen(true)}
            >
              <span className="sr-only">Open main menu</span>
              <Bars3Icon className="h-6 w-6" aria-hidden="true" />
            </button>
          </div>
          <Popover.Group className="hidden lg:flex lg:gap-x-12">
            <div
              className="relative inline-block text-left"
              onMouseEnter={() => setIsDropDownBlockchain(true)}
              onMouseLeave={() => setIsDropDownBlockchain(false)}
            >
              <div className="flex items-center gap-x-1 text-sm font-semibold leading-6 text-gray-300 cursor-pointer hover:text-[#ffa729] transition-colors">
                Blockchain
                <ChevronDownIcon className="h-5 w-5 flex-none text-gray-400" aria-hidden="true" />
              </div>

              <Transition
                show={IsDropDownBlockchain}
                as={Fragment}
                enter="transition ease-out duration-200"
                enterFrom="opacity-0 translate-y-1"
                enterTo="opacity-100 translate-y-0"
                leave="transition ease-in duration-150"
                leaveFrom="opacity-100 translate-y-0"
                leaveTo="opacity-0 translate-y-1"
              >
                <div className="absolute -left-8 top-full z-10 w-screen max-w-md overflow-hidden rounded-xl bg-[#2d2d2d] shadow-lg">
                  <div className="p-4">
                    {blockchain.map((item) => (
                      <Link key={item.name} href={item.href} passHref>
                        <p className="group relative flex items-center gap-x-6 rounded-lg p-4 text-sm leading-6 hover:bg-[#3d3d3d] transition-colors w-full">
                          <div className="flex h-11 w-11 flex-none items-center justify-center rounded-lg bg-[#1a1a1a] group-hover:bg-[#2d2d2d]">
                            {item.imgSrc ? (
                              <Image src={item.imgSrc} width={24} height={24} alt={item.name} />
                            ) : (
                              item.icon && <item.icon className="h-6 w-6 text-gray-300 group-hover:text-[#ffa729]" aria-hidden="true" />
                            )}
                          </div>
                          <div className="flex-auto">
                            <span className="block font-semibold text-gray-300">{item.name}</span>
                            <p className="mt-1 text-gray-400">{item.description}</p>
                          </div>
                        </p>
                      </Link>
                    ))}
                  </div>
                </div>
              </Transition>
            </div>

            <div
              className="relative inline-block text-left"
              onMouseEnter={() => setisDropDownTools(true)}
              onMouseLeave={() => setisDropDownTools(false)}
            >
              <div className="flex items-center gap-x-1 text-sm font-semibold leading-6 text-gray-300 cursor-pointer hover:text-[#ffa729] transition-colors">
                Tools
                <ChevronDownIcon className="h-5 w-5 flex-none text-gray-400" aria-hidden="true" />
              </div>

              <Transition
                show={isDropDownTools}
                as={Fragment}
                enter="transition ease-out duration-200"
                enterFrom="opacity-0 translate-y-1"
                enterTo="opacity-100 translate-y-0"
                leave="transition ease-in duration-150"
                leaveFrom="opacity-100 translate-y-0"
                leaveTo="opacity-0 translate-y-1"
              >
                <div className="absolute -left-8 top-full z-10 w-screen max-w-md overflow-hidden rounded-xl bg-[#2d2d2d] shadow-lg">
                  <div className="p-4">
                    {tools.map((item) => (
                      <Link key={item.name} href={item.href} passHref>
                        <p className="group relative flex items-center gap-x-6 rounded-lg p-4 text-sm leading-6 hover:bg-[#3d3d3d] transition-colors w-full">
                          <div className="flex h-11 w-11 flex-none items-center justify-center rounded-lg bg-[#1a1a1a] group-hover:bg-[#2d2d2d]">
                            {item.imgSrc ? (
                              <Image src={item.imgSrc} width={24} height={24} alt={item.name} layout="fixed" />
                            ) : (
                              item.icon && <item.icon className="h-6 w-6 text-gray-300 group-hover:text-[#ffa729]" aria-hidden="true" />
                            )}
                          </div>
                          <div className="flex-auto">
                            <span className="block font-semibold text-gray-300">{item.name}</span>
                            <p className="mt-1 text-gray-400">{item.description}</p>
                          </div>
                        </p>
                      </Link>
                    ))}
                  </div>
                </div>
              </Transition>
            </div>

            <Link href="/richlist" className="text-sm font-semibold leading-6 text-gray-300 hover:text-[#ffa729] transition-colors">
              Richlist
            </Link>
          </Popover.Group>
          <AuthProfileIcon />
        </nav>
        <Dialog as="div" className="lg:hidden" open={mobileMenuOpen} onClose={setMobileMenuOpen}>
          <div className="fixed inset-0 z-10" />
          <Dialog.Panel className="fixed inset-y-0 right-0 z-10 w-full overflow-y-auto bg-[#1a1a1a] px-6 py-6 sm:max-w-sm sm:ring-1 sm:ring-gray-900/10">
            <div className="flex items-center justify-between">
              <Link href="/" className="-m-1.5 p-1.5" onClick={(e) => setMobileMenuOpen(false)}>
                <span className="sr-only">Quanta Explorer</span>
                <Image className="h-8 w-auto" src={QRLFavicon} alt="QRL Favicon" layout="fixed" />
              </Link>
              <button
                type="button"
                className="-m-2.5 rounded-md p-2.5 text-gray-300"
                onClick={() => setMobileMenuOpen(false)}
              >
                <span className="sr-only">Close menu</span>
                <XMarkIcon className="h-6 w-6" aria-hidden="true" />
              </button>
            </div>
            <div className="mt-6 flow-root">
              <div className="-my-6 divide-y divide-gray-500/10">
                <div className="space-y-2 py-6">
                  <Disclosure as="div" className="-mx-3">
                    {({ open }) => (
                      <>
                        <Disclosure.Button className="flex w-full items-center justify-between rounded-lg py-2 pl-3 pr-3.5 text-base font-semibold leading-7 text-gray-300 hover:bg-[#2d2d2d]">
                          Blockchain
                          <ChevronDownIcon
                            className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 flex-none')}
                            aria-hidden="true"
                          />
                        </Disclosure.Button>
                        <Disclosure.Panel className="mt-2 space-y-2">
                          {blockchain.map((item) => (
                            <Link
                              key={item.name}
                              href={item.href}
                              className="block rounded-lg py-2 pl-6 pr-3 text-sm font-semibold leading-7 text-gray-300 hover:bg-[#2d2d2d]"
                              onClick={() => setMobileMenuOpen(false)}
                            >
                              {item.name}
                            </Link>
                          ))}
                        </Disclosure.Panel>
                      </>
                    )}
                  </Disclosure>
                </div>

                <div className="py-6">
                  <Disclosure as="div" className="-mx-3">
                    {({ open }) => (
                      <>
                        <Disclosure.Button className="flex w-full items-center justify-between rounded-lg py-2 pl-3 pr-3.5 text-base font-semibold leading-7 text-gray-300 hover:bg-[#2d2d2d]">
                          Tools
                          <ChevronDownIcon
                            className={classNames(open ? 'rotate-180' : '', 'h-5 w-5 flex-none')}
                            aria-hidden="true"
                          />
                        </Disclosure.Button>
                        <Disclosure.Panel className="mt-2 space-y-2">
                          {tools.map((item) => (
                            <Link
                              key={item.name}
                              href={item.href}
                              className="block rounded-lg py-2 pl-6 pr-3 text-sm font-semibold leading-7 text-gray-300 hover:bg-[#2d2d2d]"
                              onClick={() => setMobileMenuOpen(false)}
                            >
                              {item.name}
                            </Link>
                          ))}
                        </Disclosure.Panel>
                      </>
                    )}
                  </Disclosure>

                  <Link
                    href="/richlist"
                    className="-mx-3 block rounded-lg px-3 py-2 text-base font-semibold leading-7 text-gray-300 hover:bg-[#2d2d2d]"
                    onClick={() => setMobileMenuOpen(false)}
                  >
                    Richlist
                  </Link>
                </div>
              </div>
            </div>
          </Dialog.Panel>
        </Dialog>
      </header>
    </>
  );
}
