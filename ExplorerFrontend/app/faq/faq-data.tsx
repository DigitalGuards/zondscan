import React from 'react';
import Link from 'next/link';

const LINK =
  'text-accent hover:text-accent-hover underline decoration-accent/40 underline-offset-2 transition-colors';

export interface FaqItem {
  id: string;
  q: string;
  a: React.ReactNode;
  aText: string;
}

export const faqs: FaqItem[] = [
  {
    id: 'what-is-zondscan',
    q: 'What is ZondScan?',
    a: (
      <>
        ZondScan is the block explorer for the QRL 2.0 network. Use it to
        search and inspect blocks, transactions, addresses,{' '}
        <Link href="/validators" className={LINK}>
          validators
        </Link>
        , and smart contracts. It also bundles developer tools, including a
        contract verifier, an open REST API, a{' '}
        <Link href="/gas" className={LINK}>
          gas tracker
        </Link>
        , a{' '}
        <Link href="/converter" className={LINK}>
          unit converter
        </Link>
        , and a testnet faucet.
      </>
    ),
    aText:
      'ZondScan is the block explorer for the QRL 2.0 network. Use it to search and inspect blocks, transactions, addresses, validators, and smart contracts. It also bundles developer tools, including a contract verifier, an open REST API, a gas tracker, a unit converter, and a testnet faucet.',
  },
  {
    id: 'what-is-qrl-2',
    q: 'What is QRL 2.0?',
    a: (
      <>
        QRL 2.0 is the next generation of the Quantum Resistant Ledger: a
        proof of stake, EVM compatible blockchain where every account is
        protected by post-quantum signatures. Because the execution layer is
        EVM compatible, smart contracts and familiar Ethereum tooling carry
        over. The network currently runs as a public testnet; mainnet arrives
        after the migration from the legacy QRL chain. Read the full
        introduction in{' '}
        <Link href="/learn/what-is-qrl-2" className={LINK}>
          What is QRL 2.0
        </Link>
        .
      </>
    ),
    aText:
      'QRL 2.0 is the next generation of the Quantum Resistant Ledger: a proof of stake, EVM compatible blockchain where every account is protected by post-quantum signatures. Because the execution layer is EVM compatible, smart contracts and familiar Ethereum tooling carry over. The network currently runs as a public testnet; mainnet arrives after the migration from the legacy QRL chain.',
  },
  {
    id: 'quantum-resistant',
    q: 'What makes QRL 2.0 quantum resistant?',
    a: (
      <>
        Quantum computers running Shor&apos;s algorithm are expected to break
        the elliptic curve signatures (ECDSA) that secure most existing
        blockchains. QRL 2.0 signs every transaction with ML-DSA-87
        (CRYSTALS-Dilithium), a lattice based scheme standardized by NIST in
        FIPS 204 and designed to withstand both classical and quantum attacks.
        The legacy QRL chain pioneered post-quantum signing with XMSS (RFC
        8391), a stateful hash based scheme; QRL 2.0 adopts ML-DSA-87 for its
        account model. Learn more in{' '}
        <Link href="/learn/why-post-quantum" className={LINK}>
          why post-quantum matters
        </Link>
        .
      </>
    ),
    aText:
      'Quantum computers running Shor’s algorithm are expected to break the elliptic curve signatures (ECDSA) that secure most existing blockchains. QRL 2.0 signs every transaction with ML-DSA-87 (CRYSTALS-Dilithium), a lattice based scheme standardized by NIST in FIPS 204 and designed to withstand both classical and quantum attacks. The legacy QRL chain pioneered post-quantum signing with XMSS (RFC 8391), a stateful hash based scheme; QRL 2.0 adopts ML-DSA-87 for its account model.',
  },
  {
    id: 'track-transaction',
    q: 'How do I track a transaction?',
    a: (
      <>
        Paste a transaction hash into the search bar at the top of any page to
        jump straight to its detail view. The transaction page shows status,
        block, value in Quanta, gas usage, fees, and the sender and recipient
        addresses. Transactions that have been broadcast but are still waiting
        to be mined appear in the{' '}
        <Link href="/pending/1" className={LINK}>
          pending view
        </Link>
        , and you can browse recent activity on the{' '}
        <Link href="/transactions" className={LINK}>
          transactions page
        </Link>
        . For a field by field walkthrough, see{' '}
        <Link href="/learn/read-a-transaction" className={LINK}>
          how to read a transaction
        </Link>
        .
      </>
    ),
    aText:
      'Paste a transaction hash into the search bar at the top of any page to jump straight to its detail view. The transaction page shows status, block, value in Quanta, gas usage, fees, and the sender and recipient addresses. Transactions that have been broadcast but are still waiting to be mined appear in the pending view, and you can browse recent activity on the transactions page.',
  },
  {
    id: 'testnet-quanta',
    q: 'How do I get testnet Quanta?',
    a: (
      <>
        Use the built in{' '}
        <Link href="/faucet" className={LINK}>
          faucet
        </Link>{' '}
        to request free testnet Quanta for any Q-prefixed address. Testnet
        coins have no monetary value; they exist so you can try transactions,
        contracts, and tooling without risk. The faucet sends a fixed amount
        per request with a cooldown between requests for the same address.
        Step by step instructions are in{' '}
        <Link href="/learn/get-testnet-quanta" className={LINK}>
          get testnet Quanta
        </Link>
        .
      </>
    ),
    aText:
      'Use the built in faucet at zondscan.com/faucet to request free testnet Quanta for any Q-prefixed address. Testnet coins have no monetary value; they exist so you can try transactions, contracts, and tooling without risk. The faucet sends a fixed amount per request with a cooldown between requests for the same address.',
  },
  {
    id: 'api-key',
    q: 'Does the ZondScan API require an API key?',
    a: (
      <>
        No. The ZondScan API is free and requires no signup, key, or
        authentication. GET endpoints are open to any origin via CORS, so you
        can call them directly from a browser application. Browse every
        endpoint and run live requests in the{' '}
        <Link href="/api-explorer" className={LINK}>
          API explorer
        </Link>
        .
      </>
    ),
    aText:
      'No. The ZondScan API is free and requires no signup, key, or authentication. GET endpoints are open to any origin via CORS, so you can call them directly from a browser application. Browse every endpoint and run live requests in the API explorer at zondscan.com/api-explorer.',
  },
  {
    id: 'smart-contracts',
    q: 'What can I see about smart contracts?',
    a: (
      <>
        The{' '}
        <Link href="/contracts" className={LINK}>
          contracts page
        </Link>{' '}
        lists deployed contracts the indexer has seen, including QRC20 tokens
        and NFT collections. Each contract address page shows the creation
        transaction, balance, transaction history, and token transfers.
        Verified contracts additionally expose their full source code,
        compiler settings, and ABI, and you can call their view and pure
        functions directly from the Read tab. Verified contracts also get an
        AI generated plain language explanation of what the code does.
      </>
    ),
    aText:
      'The contracts page lists deployed contracts the indexer has seen, including QRC20 tokens and NFT collections. Each contract address page shows the creation transaction, balance, transaction history, and token transfers. Verified contracts additionally expose their full source code, compiler settings, and ABI, and you can call their view and pure functions directly from the Read tab. Verified contracts also get an AI generated plain language explanation of what the code does.',
  },
  {
    id: 'verify-contract',
    q: 'How do I verify my contract source code?',
    a: (
      <>
        Open the{' '}
        <Link href="/verify-contract" className={LINK}>
          verify contract page
        </Link>
        , paste your source code, pick the exact Hyperion (hypc) compiler
        build you deployed with, and set the same optimizer settings. ZondScan
        recompiles the source and byte-matches the result against the runtime
        code on chain. On success the source, ABI, and compiler settings are
        published on the contract&apos;s address page. A full walkthrough from
        deployment to verification is in{' '}
        <Link href="/learn/deploy-and-verify-contract" className={LINK}>
          deploy and verify a contract
        </Link>
        .
      </>
    ),
    aText:
      'Open the verify contract page, paste your source code, pick the exact Hyperion (hypc) compiler build you deployed with, and set the same optimizer settings. ZondScan recompiles the source and byte-matches the result against the runtime code on chain. On success the source, ABI, and compiler settings are published on the contract’s address page.',
  },
  {
    id: 'qrl-vs-ethereum',
    q: 'How is QRL 2.0 different from Ethereum?',
    a: (
      <>
        QRL 2.0 keeps the developer experience of Ethereum: the execution
        layer, gqrl, comes from the go-ethereum lineage and is fully EVM
        compatible, and consensus runs on a proof of stake beacon chain
        (qrysm, a fork of Prysm). The signature layer is where the two chains
        diverge. Ethereum secures accounts with ECDSA. QRL 2.0 secures them
        with the post-quantum ML-DSA-87 scheme, uses Q-prefixed addresses, and
        denominates amounts in Quanta (1 Quanta = 10<sup>9</sup> Shor = 10
        <sup>18</sup> Planck); see the{' '}
        <Link href="/learn/quanta-shor-planck" className={LINK}>
          unit guide
        </Link>{' '}
        and the{' '}
        <Link href="/converter" className={LINK}>
          converter
        </Link>
        .
      </>
    ),
    aText:
      'QRL 2.0 keeps the developer experience of Ethereum: the execution layer, gqrl, comes from the go-ethereum lineage and is fully EVM compatible, and consensus runs on a proof of stake beacon chain (qrysm, a fork of Prysm). The signature layer is where the two chains diverge. Ethereum secures accounts with ECDSA. QRL 2.0 secures them with the post-quantum ML-DSA-87 scheme, uses Q-prefixed addresses, and denominates amounts in Quanta (1 Quanta = 10^9 Shor = 10^18 Planck).',
  },
  {
    id: 'scaling',
    q: 'Can QRL 2.0 scale beyond the base layer?',
    a: (
      <>
        The base layer settles about 950 transfers per block (a 20 million
        gas cap at 21,000 gas each), and every one of them carries a 7.2 KB
        ML-DSA-87 signature envelope. The QRL 2.0 virtual machine has no
        pairing precompiles, so the validity proofs it can verify are
        hash-based STARKs, which are post-quantum by construction. QuantaStark, a DigitalGuards
        research project, measured a Hyperion STARK verifier at 2.2 to 4.4
        million gas per proof on a local QRL 2.0 network, enough for a
        validity rollup at roughly 540 gas per transfer. Read{' '}
        <Link href="/learn/scaling-qrl-with-stark-proofs" className={LINK}>
          scaling QRL 2.0 with post-quantum STARK proofs
        </Link>
        .
      </>
    ),
    aText:
      'The base layer settles about 950 transfers per block (a 20 million gas cap at 21,000 gas each), and every one of them carries a 7.2 KB ML-DSA-87 signature envelope. The QRL 2.0 virtual machine has no pairing precompiles, so the validity proofs it can verify are hash-based STARKs, which are post-quantum by construction. QuantaStark, a DigitalGuards research project, measured a Hyperion STARK verifier at 2.2 to 4.4 million gas per proof on a local QRL 2.0 network, enough for a validity rollup at roughly 540 gas per transfer.',
  },
  {
    id: 'which-wallet',
    q: 'Which wallet should I use?',
    a: (
      <>
        MyQRLWallet, available at{' '}
        <a
          href="https://qrlwallet.com"
          target="_blank"
          rel="noopener noreferrer"
          className={LINK}
        >
          qrlwallet.com
        </a>
        , is the open source wallet for QRL 2.0. It runs in the browser, with
        desktop and mobile apps available and a browser extension in
        preparation. It manages
        Q-prefixed accounts, sends Quanta and QRC20 tokens, and connects to
        dApps. Follow the step by step guide to{' '}
        <Link href="/learn/create-a-wallet" className={LINK}>
          create a wallet
        </Link>
        .
      </>
    ),
    aText:
      'MyQRLWallet, available at qrlwallet.com, is the open source wallet for QRL 2.0. It runs in the browser, with desktop and mobile apps available and a browser extension in preparation. It manages Q-prefixed accounts, sends Quanta and QRC20 tokens, and connects to dApps.',
  },
  {
    id: 'who-builds-zondscan',
    q: 'Who builds ZondScan?',
    a: (
      <>
        ZondScan was created by DigitalGuards, a company based in the
        Netherlands. The explorer is completely open source and its code is
        available on GitHub at{' '}
        <a
          href="https://github.com/DigitalGuards/zondscan"
          target="_blank"
          rel="noopener noreferrer"
          className={LINK}
        >
          github.com/DigitalGuards/zondscan
        </a>
        . You can learn more about DigitalGuards at{' '}
        <a
          href="https://digitalguards.nl/"
          target="_blank"
          rel="noopener noreferrer"
          className={LINK}
        >
          digitalguards.nl
        </a>
        .
      </>
    ),
    aText:
      'ZondScan was created by DigitalGuards, a company based in the Netherlands. The explorer is completely open source and its code is available on GitHub at github.com/DigitalGuards/zondscan. You can learn more about DigitalGuards at digitalguards.nl.',
  },
  {
    id: 'support',
    q: 'How do I report a problem or get support?',
    a: (
      <>
        For technical support and explorer issues, email{' '}
        <a href="mailto:info@digitalguards.nl" className={LINK}>
          info@digitalguards.nl
        </a>
        . For wallet and general inquiries, email{' '}
        <a href="mailto:info@qrlwallet.com" className={LINK}>
          info@qrlwallet.com
        </a>
        . You can also join the QRL community on{' '}
        <a
          href="https://discord.com/invite/XxJtvMuy6m"
          target="_blank"
          rel="noopener noreferrer"
          className={LINK}
        >
          Discord
        </a>{' '}
        for real time discussion and support. Bug reports and feature requests
        are welcome as issues on the{' '}
        <a
          href="https://github.com/DigitalGuards/zondscan"
          target="_blank"
          rel="noopener noreferrer"
          className={LINK}
        >
          ZondScan repository
        </a>
        .
      </>
    ),
    aText:
      'For technical support and explorer issues, email info@digitalguards.nl. For wallet and general inquiries, email info@qrlwallet.com. You can also join the QRL community on Discord for real time discussion and support. Bug reports and feature requests are welcome as issues on the ZondScan GitHub repository.',
  },
];
