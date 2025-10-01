'use client'

import React from 'react'
import { Disclosure } from '@headlessui/react'
import { ChevronDownIcon } from '@heroicons/react/20/solid'

// FAQ data structure
const faqs = [
  {
    question: "What is Zondscan?",
    answer: "Zondscan is a block explorer for the QRL Zond blockchain. It allows you to explore and search the blockchain for transactions, blocks, addresses, and smart contracts. As a fork of Ethereum modified for quantum resistance, Zondscan provides similar functionality to Etherscan but for the QRL Zond network."
  },
  {
    question: "What makes QRL Zond quantum resistant?",
    answer: "QRL Zond uses SPHINCS+, a NIST-standardized stateless hash-based signature scheme. Traditional blockchains use ECDSA signatures, which quantum computers will break. SPHINCS+ is designed to withstand quantum attacks. The vulnerability isn't SHA-256 hashing, it's the ECDSA signature scheme that protects transactions. When quantum computers break ECDSA, attackers can forge transactions. SPHINCS+ prevents this."
  },
  {
    question: "How can I use Zondscan to track transactions?",
    answer: "You can track transactions in several ways:\n\n1. Use the search bar to look up specific transaction hashes\n2. View the latest transactions on the home page or transactions page\n3. Track transactions related to specific addresses\n4. View pending transactions in the mempool\n\nEach transaction page shows detailed information including status, value transferred, gas fees, and involved addresses."
  },
  {
    question: "What information can I find about smart contracts?",
    answer: "Zondscan provides comprehensive information about smart contracts:\n\n1. Contract source code (if verified)\n2. Contract creation transaction\n3. Contract interactions and internal transactions\n4. Contract balance and token holdings\n5. Contract events and logs\n\nVerified contracts also include their ABI and can be interacted with directly through the explorer."
  },
  {
    question: "How do I verify smart contract source code?",
    answer: "To verify a smart contract on Zondscan:\n\n1. Navigate to the contract address page\n2. Click on the 'Verify & Publish' button\n3. Upload the original source code\n4. Provide the exact compiler version and optimization settings used\n5. Submit for verification\n\nOnce verified, the contract's source code and interactions become publicly visible on Zondscan."
  },
  {
    question: "What are the differences between QRL Zond and Ethereum?",
    answer: "QRL Zond is based on Ethereum with key changes:\n\n1. Uses SPHINCS+ signatures instead of ECDSA\n2. Modified transaction format for quantum-resistant signatures\n3. Adjusted gas calculations for quantum-resistant operations\n4. Full EVM compatibility for smart contracts\n\nMost Ethereum tools and development practices work the same."
  },
  {
    question: "Who created Zondscan?",
    answer: "Zondscan was created by DigitalGuards, a company based in the Netherlands. The explorer is completely open-source and its code is available on GitHub at <a href='https://github.com/DigitalGuards/zondexplorer' target='_blank' rel='noopener noreferrer' style='color: #ffa729; border-bottom: 1px solid #ffa729; text-decoration: none;'>github.com/DigitalGuards/zondexplorer</a>. You can learn more about DigitalGuards at <a href='https://digitalguards.nl/' target='_blank' rel='noopener noreferrer' style='color: #ffa729; border-bottom: 1px solid #ffa729; text-decoration: none;'>digitalguards.nl</a>."
  },
  {
    question: "How can I contact support?",
    answer: "For support or inquiries, you can reach out to us via email:\n\n• Technical support and explorer issues: <a href='mailto:info@digitalguards.nl' style='color: #ffa729; border-bottom: 1px solid #ffa729; text-decoration: none;'>info@digitalguards.nl</a>\n• Wallet and general inquiries: <a href='mailto:info@qrlwallet.com' style='color: #ffa729; border-bottom: 1px solid #ffa729; text-decoration: none;'>info@qrlwallet.com</a>\n\nJoin the QRL community on <a href='https://discord.com/invite/XxJtvMuy6m' target='_blank' rel='noopener noreferrer' style='color: #ffa729; border-bottom: 1px solid #ffa729; text-decoration: none;'>Discord</a> for real-time discussions and support."
  }
]

function classNames(...classes: string[]) {
  return classes.filter(Boolean).join(' ')
}

export default function FAQClient() {
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
                    <span className="text-sm md:text-base font-medium text-[#ffa729]">{faq.question}</span>
                    <ChevronDownIcon
                      className={classNames(
                        open ? 'rotate-180' : '',
                        'h-5 w-5 text-[#ffa729] transition-transform duration-200'
                      )}
                    />
                  </Disclosure.Button>
                  <Disclosure.Panel className="px-4 py-3 text-sm bg-[#262626] border-t border-[#3d3d3d]" dangerouslySetInnerHTML={{ __html: faq.answer.replace(/className='text-\[#ffa729\] hover:underline'/g, "className='text-[#ffa729] underline decoration-[#ffa729] hover:opacity-80'") }} />
                </>
              )}
            </Disclosure>
          ))}
        </div>
      </div>
    </div>
  )
} 