import type { Metadata } from 'next';
import Link from 'next/link';
import { learnMetadata } from '../components/learn-metadata';
import LearnArticle, {
  LearnSection,
  StepList,
  Step,
  Callout,
  LearnFAQ,
} from '../components/LearnArticle';

export const metadata: Metadata = learnMetadata('what-is-qrl-2');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="what-is-qrl-2">
      <p>
        QRL 2.0 is the next generation of the{' '}
        <a href="https://www.theqrl.org" target="_blank" rel="noopener noreferrer">
          Quantum Resistant Ledger
        </a>
        : a proof of stake blockchain with EVM compatible smart contracts, secured end to end
        by post-quantum signatures. Every account signs with ML-DSA-87, a lattice based scheme
        standardized by NIST, so the chain is designed to stay secure even against attackers
        equipped with large quantum computers.
      </p>
      <p>
        This article gives you the full picture in one sitting: what QRL 2.0 is technically,
        how it relates to the legacy QRL chain that has been running since 2018, what you can
        already do on the public testnet today, and how ZondScan, the explorer you are reading
        this on, fits into the ecosystem.
      </p>
      <p>
        No prior blockchain experience is required. Where a topic deserves more depth, we link
        to a dedicated guide, starting with{' '}
        <a href="/learn/why-post-quantum">why post-quantum cryptography matters</a>.
      </p>

      <LearnSection id="the-short-version" title="QRL 2.0, the short version">
        <p>
          Under the hood, QRL 2.0 runs the same two-layer architecture as modern proof of stake
          Ethereum, with two clients working together:
        </p>
        <ul>
          <li>
            <strong>Consensus layer:</strong> a proof of stake beacon chain run by{' '}
            <code>qrysm</code>, a fork of the Prysm client. Validators stake Quanta, propose
            blocks, and attest to the chain in slots and epochs.
          </li>
          <li>
            <strong>Execution layer:</strong> <code>gqrl</code>, a client of go-ethereum
            lineage. It executes transactions and EVM smart contracts, so concepts like gas,
            contract bytecode, and token standards carry over.
          </li>
          <li>
            <strong>Signatures:</strong> ML-DSA-87, the strongest parameter set of ML-DSA
            (the standardized form of CRYSTALS-Dilithium),
            standardized as NIST FIPS 204. This replaces the elliptic curve signatures that
            most chains rely on and that quantum algorithms are expected to break.
          </li>
        </ul>
        <p>
          Addresses on QRL 2.0 start with <code>Q</code>. Amounts are denominated in{' '}
          <strong>Quanta</strong>: one Quanta is 10^9 Shor and 10^18 Planck. The{' '}
          <a href="/learn/quanta-shor-planck">units guide</a> covers the conversions, and the
          built in <Link href="/converter">converter</Link> does the arithmetic for you.
        </p>
      </LearnSection>

      <LearnSection id="two-chains-one-project" title="Two chains, one project">
        <p>
          The QRL project currently maintains two networks. The legacy QRL chain has run in
          production since 2018 and is secured by XMSS (RFC 8391), a hash based signature
          scheme. XMSS is stateful: each key can produce a limited number of signatures and the
          wallet must track which ones have been used. It has protected the ledger reliably for
          years, and it predates the NIST post-quantum standards.
        </p>
        <p>
          QRL 2.0 is the redesign. It moves consensus to proof of stake, adds EVM smart
          contracts, and adopts ML-DSA-87, which is stateless and fits the account model that
          smart contract chains use. QRL 2.0 currently runs as a <strong>public testnet</strong>.
          Mainnet arrives after the migration from the legacy chain. The official project
          documentation lives at{' '}
          <a href="https://docs.theqrl.org" target="_blank" rel="noopener noreferrer">
            docs.theqrl.org
          </a>
          .
        </p>
        <Callout type="warning" title="Testnet coins have no monetary value">
          <p>
            Everything you see on ZondScan today is testnet activity. Quanta on the QRL 2.0
            testnet are free from the <Link href="/faucet">faucet</Link> and are worth nothing. Never
            pay anyone for testnet coins, and never send real funds to a testnet address.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="what-you-can-do-today" title="What you can do today">
        <p>
          The testnet is a full environment, so the entire product stack around QRL 2.0 is
          already usable. A reasonable first tour looks like this:
        </p>
        <StepList>
          <Step title="Create a wallet">
            <p>
              <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">
                MyQRLWallet
              </a>{' '}
              runs in the browser and generates post-quantum accounts with Q-prefixed
              addresses. The <a href="/learn/create-a-wallet">wallet guide</a> walks through
              setup and, most importantly, backing up your seed.
            </p>
          </Step>
          <Step title="Get testnet Quanta">
            <p>
              The <Link href="/faucet">ZondScan faucet</Link> sends free testnet Quanta to any
              address. See <a href="/learn/get-testnet-quanta">the faucet guide</a> for the
              short walkthrough.
            </p>
          </Step>
          <Step title="Send a transaction and read it back">
            <p>
              Transfer some Quanta between your own addresses, then find the transaction on the{' '}
              <Link href="/transactions">transactions list</Link> or by pasting its hash into the
              search bar. <a href="/learn/read-a-transaction">Reading a transaction</a> explains
              every field on the page.
            </p>
          </Step>
          <Step title="Deploy a smart contract or launch a token">
            <p>
              Contracts are compiled with Hyperion and deployed to the EVM compatible execution
              layer. You can <a href="/learn/deploy-and-verify-contract">deploy and verify a
              contract</a> so its source appears on ZondScan, or{' '}
              <a href="/learn/launch-a-qrc20">launch a QRC20 token</a> straight from your
              wallet.
            </p>
          </Step>
          <Step title="Stake and swap">
            <p>
              <a href="https://quantapool.com" target="_blank" rel="noopener noreferrer">
                QuantaPool
              </a>{' '}
              offers liquid staking: you stake Quanta and receive a liquid staking token in
              return (<a href="/learn/stake-with-quantapool">guide</a>).{' '}
              <a href="https://quantaswap.io" target="_blank" rel="noopener noreferrer">
                QuantaSwap
              </a>{' '}
              runs atomic swaps between testnet Quanta and Sepolia ETH using hash time locked
              contracts, with no custodian in the middle (
              <a href="/learn/swap-with-quantaswap">guide</a>).
            </p>
          </Step>
        </StepList>
        <p>
          For a wider map of what exists around the chain, the community-maintained{' '}
          <a href="https://www.qrlecosystem.com/" target="_blank" rel="noopener noreferrer">
            QRL Ecosystem Index
          </a>{' '}
          catalogs projects, tools, services, and resources across QRL 1.x and QRL 2.0.
        </p>
      </LearnSection>

      <LearnSection id="where-zondscan-fits" title="Where ZondScan fits">
        <p>
          ZondScan is the block explorer for QRL 2.0. A synchronizer reads every block from a
          network node and indexes it into a database, and this site serves that data with
          search across blocks, transactions, and addresses. From here you can inspect:
        </p>
        <ul>
          <li>
            <strong>Blocks and transactions</strong>, including{' '}
            <Link href="/pending/1">pending transactions</Link> still waiting in the mempool.
          </li>
          <li>
            <strong>Addresses</strong>, with native balance, QRC20 token holdings and
            transfers, NFTs, and internal contract calls.
          </li>
          <li>
            <strong>Validators and epochs</strong>: the <Link href="/validators">validator
            list</Link> and per-epoch data, explained in{' '}
            <a href="/learn/validators-and-epochs">validators and epochs</a>.
          </li>
          <li>
            <strong>Smart contracts</strong>: the <Link href="/contracts">contracts list</Link> and
            the <Link href="/verify-contract">verification flow</Link>, which recompiles submitted
            source with a pinned Hyperion build and byte-matches it against the on-chain code
            before publishing it.
          </li>
          <li>
            <strong>Network stats and tools</strong>: live block height, validator count, and
            staked Quanta on the home page, plus a <Link href="/gas">gas tracker</Link>, the{' '}
            <Link href="/richlist">richlist</Link>, and a{' '}
            <Link href="/api-explorer">REST API explorer</Link> for programmatic access.
          </li>
        </ul>
        <p>
          The whole stack is open source under the MIT license at{' '}
          <a
            href="https://github.com/DigitalGuards/zondscan"
            target="_blank"
            rel="noopener noreferrer"
          >
            github.com/DigitalGuards/zondscan
          </a>
          .
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Is QRL 2.0 live?',
            a: (
              <p>
                It runs as a public testnet that anyone can use today. Mainnet follows the
                migration from the legacy QRL chain. Until then, all coins and activity on QRL
                2.0 are for testing.
              </p>
            ),
          },
          {
            q: 'Do I need to buy anything to try it?',
            a: (
              <p>
                No. The <Link href="/faucet">faucet</Link> dispenses testnet Quanta for free, and
                they have no monetary value. Wallets, staking, swaps, and contract deployment
                all work with faucet funds.
              </p>
            ),
          },
          {
            q: 'Does my Ethereum knowledge carry over?',
            a: (
              <p>
                Largely, yes. The execution layer runs the EVM, so gas, contract bytecode, and
                token standards behave the way you expect, and contracts are compiled with
                Hyperion. Accounts differ: keys use ML-DSA-87 and addresses are Q-prefixed, so
                you need a QRL wallet such as{' '}
                <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">
                  MyQRLWallet
                </a>{' '}
                to hold coins and sign transactions.
              </p>
            ),
          },
          {
            q: 'Why does QRL use post-quantum signatures?',
            a: (
              <p>
                A sufficiently large quantum computer could recover private keys from the
                elliptic curve signatures most blockchains use today. QRL builds on
                NIST-standardized post-quantum schemes from the start.{' '}
                <a href="/learn/why-post-quantum">Why post-quantum</a> covers the threat model
                in detail.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
