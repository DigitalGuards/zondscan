'use client'

import React from 'react'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon } from '@heroicons/react/20/solid'

// FAQ data structure
const faqs = [
  {
    question: "What makes QRL quantum resistant?",
    answer: "QRL uses XMSS (eXtended Merkle Signature Scheme), which is a post-quantum secure digital signature scheme. Unlike traditional blockchains that use ECDSA (which will be broken by quantum computers), XMSS is designed to be resistant to quantum attacks. The threat isn't about SHA-256 hashing - it's about the signature scheme ECDSA, which will be broken much earlier and would allow forging of transactions."
  },
  {
    question: "Haven't other blockchains already planned for quantum computing?",
    answer: "While some blockchains have discussed quantum resistance, their current plans face severe challenges:\n\n1. Soft fork solutions would still leave millions of coins in vulnerable addresses\n2. Simply using longer signatures (like upgrading from 256 to 384-bit curves) only delays the problem by months\n3. Lost addresses (estimated at 20% for some chains) can never be upgraded\n4. Any solution requiring all users to move their funds will never achieve 100% participation"
  },
  {
    question: "Can't blockchains just hard fork to become quantum resistant?",
    answer: "Hard forking to quantum resistance isn't as simple as it sounds:\n\n1. Option 1: Making old signatures unusable would lock all existing coins forever\n2. Option 2A: Creating a new chain where users must register beforehand excludes users who don't register in time\n3. Option 2B: Allowing users to claim new coins by proving ownership of old ones remains vulnerable to quantum attacks\n4. Any fork creates market uncertainty and potential value instability\n\nQRL avoids these issues by being quantum resistant from the start."
  },
  {
    question: "Isn't quantum computing too far away to worry about?",
    answer: "The threat is closer and more serious than many realize:\n\n1. Transactions are vulnerable during the entire period they're in the mempool and during block confirmation\n2. According to research, quantum computers could be powerful enough to break ECDSA during a block time window as early as 2027\n3. 'Store now, decrypt later' attacks mean quantum-vulnerable transactions today could be broken when quantum computers arrive\n4. Network congestion can leave transactions vulnerable for hours in the mempool"
  },
  {
    question: "How do I keep my wallet secure?",
    answer: "1. Never share your private key or seed phrase with anyone\n2. Store your seed phrase offline in a secure location\n3. Use a strong password for your wallet\n4. Always verify transaction details before sending\n5. Be cautious of phishing attempts and only use official QRL websites"
  },
  {
    question: "What should I do if I lose my wallet access?",
    answer: "If you have saved your seed phrase, you can recover your wallet by using the 'Restore Wallet' option and entering your seed phrase. If you've lost both your wallet access and seed phrase, unfortunately, there is no way to recover your funds. This emphasizes the importance of securely storing your seed phrase."
  }
]

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function FAQPage() {
  return (
    <div className="min-h-screen bg-[#1a1a1a] text-gray-300 p-4 md:p-8">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-2xl md:text-3xl font-bold mb-8 text-[#ffa729]">
          Frequently Asked Questions
        </h1>
        
        <div className="space-y-4">
          {faqs.map((faq, index) => (
            <Disclosure as="div" key={index} className="bg-[#2d2d2d] rounded-lg overflow-hidden">
              {({ open }) => (
                <>
                  <Disclosure.Button className="flex w-full items-center justify-between px-4 py-3 text-left">
                    <span className="text-sm md:text-base font-medium">{faq.question}</span>
                    <ChevronDownIcon
                      className={classNames(
                        open ? 'rotate-180' : '',
                        'h-5 w-5 text-[#ffa729] transition-transform duration-200'
                      )}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="px-4 py-3 text-sm bg-[#262626] border-t border-[#3d3d3d]">
                    {faq.answer}
                  </Disclosure.Panel>
                </>
              )}
            </Disclosure>
          ))}
        </div>
      </div>
    </div>
  )
}
