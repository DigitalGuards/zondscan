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
  // { name: 'Unconfirmed Transactions', description: 'View all your unconfirmed Transactions', href: '/transactions', icon: SquaresPlusIcon, imgSrc: LoadingIcon },
  { name: 'View Transactions', description: 'Here you can view all the Transactions', href: '/transactions/1', icon: SquaresPlusIcon, imgSrc: PartnerHandshakeIcon },
  { name: 'View Blocks', description: 'Here you can view all the Blocks', href: '/blocks/1', imgSrc: BlockchainIcon },
  { name: 'View Contracts', description: 'Explore all QRL contracts', href: '/contracts', imgSrc: ContractIcon },
  // { name: 'Richlist', description: 'View your rank on the richlist', href: '/richlist', icon: CursorArrowRaysIcon, imgSrc: null },
]

const tools = [
  { name: 'Account Balance Checker', description: 'Check your Account balance on a certain date or block', href: '/checker', icon: null, imgSrc: LookUpIcon },
  { name: 'Unit Converter', description: 'Convert QRL <-> Any currency', href: '/converter', icon: ArrowPathIcon, imgSrc: null },
]

// const tokens = [
//   { name: 'Top Tokens', description: 'Top Tokens - all in one list', href: '/toptokens', icon: ArrowPathIcon, imgSrc: LookUpIcon },
//   { name: 'Token Transfers', description: 'Token Transfers', href: '/tokentransfers', icon: ArrowPathIcon, imgSrc: TokenIcon },
// ]

// const callsToAction = [
//   // { name: 'Watch demo', href: '#', icon: PlayCircleIcon },
//   // { name: 'Contact sales', href: '#', icon: PhoneIcon },
// ]

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function Header() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [IsDropDownBlockchain, setIsDropDownBlockchain] = useState(false);
  const [isDropDownTools, setisDropDownTools] = useState(false);


  return (
    <>
      <header className="bg-white">
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
              className="-m-2.5 inline-flex items-center justify-center rounded-md p-2.5 text-gray-700"
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
              <div className="flex items-center gap-x-1 text-sm font-semibold leading-6 text-gray-900 cursor-pointer">
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
                <div className="absolute -left-8 top-full z-10 w-screen max-w-md overflow-hidden rounded-3xl bg-white shadow-lg ring-1 ring-black/5">
                  <div className="p-4">
                    {blockchain.map((item) => (
                      <Link key={item.name} href={item.href} passHref>
                        <p className="group relative flex items-center gap-x-6 rounded-lg p-4 text-sm leading-6 hover:bg-gray-50 w-full">
                          <div className="flex h-11 w-11 flex-none items-center justify-center rounded-lg bg-gray-50 group-hover:bg-white">
                            {item.imgSrc ? (
                              <Image src={item.imgSrc} width={24} height={24} alt={item.name} />
                            ) : (
                              item.icon && <item.icon className="h-6 w-6 text-gray-600 group-hover:text-indigo-600" aria-hidden="true" />
                            )}
                          </div>
                          <div className="flex-auto">
                            <span className="block font-semibold text-gray-900">{item.name}</span>
                            <p className="mt-1 text-gray-600">{item.description}</p>
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
              <div className="flex items-center gap-x-1 text-sm font-semibold leading-6 text-gray-900 cursor-pointer">
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
                <div className="absolute -left-8 top-full z-10 w-screen max-w-md overflow-hidden rounded-3xl bg-white shadow-lg ring-1 ring-black/5">
                  <div className="p-4">
                    {tools.map((item) => (
                      <Link key={item.name} href={item.href} passHref>
                        <p className="group relative flex items-center gap-x-6 rounded-lg p-4 text-sm leading-6 hover:bg-gray-50 w-full">
                          <div className="flex h-11 w-11 flex-none items-center justify-center rounded-lg bg-gray-50 group-hover:bg-white">
                            {item.imgSrc ? (
                              <Image src={item.imgSrc} width={24} height={24} alt={item.name} layout="fixed" />
                            ) : (
                              item.icon && <item.icon className="h-6 w-6 text-gray-600 group-hover:text-indigo-600" aria-hidden="true" />
                            )}
                          </div>
                          <div className="flex-auto">
                            <span className="block font-semibold text-gray-900">{item.name}</span>
                            <p className="mt-1 text-gray-600">{item.description}</p>
                          </div>
                        </p>
                      </Link>
                    ))}
                  </div>
                </div>
              </Transition>
            </div>


            <Link href="/richlist" className="text-sm font-semibold leading-6 text-gray-900">
              Richlist
            </Link>
          </Popover.Group>
          <AuthProfileIcon />
        </nav>
        <Dialog as="div" className="lg:hidden" open={mobileMenuOpen} onClose={setMobileMenuOpen}>
          <div className="fixed inset-0 z-10" />
          <Dialog.Panel className="fixed inset-y-0 right-0 z-10 w-full overflow-y-auto bg-white px-6 py-6 sm:max-w-sm sm:ring-1 sm:ring-gray-900/10">
            <div className="flex items-center justify-between">
              <Link href="/" className="-m-1.5 p-1.5" onClick={(e) => setMobileMenuOpen(false)}>
                <span className="sr-only">Quanta Explorer</span>
                <Image className="h-8 w-auto" src={QRLFavicon} alt="QRL Favicon" layout="fixed" />
              </Link>
              <button
                type="button"
                className="-m-2.5 rounded-md p-2.5 text-gray-700"
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
                        <Disclosure.Button className="flex w-full items-center justify-between rounded-lg py-2 pl-3 pr-3.5 text-base font-semibold leading-7 text-gray-900 hover:bg-gray-50">
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
                              className="block rounded-lg py-2 pl-6 pr-3 text-sm font-semibold leading-7 text-gray-900 hover:bg-gray-50"
                              onClick={() => setMobileMenuOpen(false)} // Add this line
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
                        <Disclosure.Button className="flex w-full items-center justify-between rounded-lg py-2 pl-3 pr-3.5 text-base font-semibold leading-7 text-gray-900 hover:bg-gray-50">
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
                              className="block rounded-lg py-2 pl-6 pr-3 text-sm font-semibold leading-7 text-gray-900 hover:bg-gray-50"
                              onClick={() => setMobileMenuOpen(false)} // Automatically close the menu upon clicking
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
                    className="-mx-3 block rounded-lg px-3 py-2 text-base font-semibold leading-7 text-gray-900 hover:bg-gray-50"
                    onClick={() => setMobileMenuOpen(false)} // Automatically close the menu upon clicking
                  >
                    Richlist
                  </Link>
                </div>
                {/* Remove the "Log In" link as requested */}
              </div>
            </div>
          </Dialog.Panel>
        </Dialog>
        <hr className="h-0.5 border-t-0 bg-black bg-opacity-10" />
      </header>
    </>
  );
}
