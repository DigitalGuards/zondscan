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

export const metadata: Metadata = learnMetadata('swap-with-quantaswap');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="swap-with-quantaswap">
      <p>
        <a href="https://quantaswap.io" target="_blank" rel="noopener noreferrer">
          QuantaSwap
        </a>{' '}
        is a self-custodial venue for swapping value between the Ethereum Sepolia testnet and
        the QRL 2.0 testnet. Every swap settles through hash time locked contracts (HTLCs)
        deployed on both chains, so coins move across chains with no custodian, no wrapped
        asset bridge, and no operator that could take your funds. The contracts have no owner,
        no pause switch, and no upgrade path.
      </p>
      <p>
        This guide explains how an atomic swap works, which pairs are live, the two wallet
        connections you need, and how the order book matches you with a counterparty. It then
        walks through a full swap step by step, including what happens when a swap is
        abandoned and funds refund.
      </p>
      <p>
        Everything here runs on test networks, so the coins involved have no monetary value
        and the flow is free to practice. New to the chain itself? Start with{' '}
        <a href="/learn/what-is-qrl-2">What is QRL 2.0?</a>
      </p>

      <Callout type="warning" title="Testnet only">
        <p>
          QuantaSwap currently runs against testnets. Amounts you swap are practice amounts
          with zero real-world value. Real-value launch waits on QRL 2.0 mainnet.
        </p>
      </Callout>

      <LearnSection id="what-is-an-atomic-swap" title="What is an atomic swap?">
        <p>
          An atomic swap lets two parties trade coins on two different chains without trusting
          each other or any middleman. The trick is a shared secret and two matching escrows.
        </p>
        <p>
          The initiator picks a random 32-byte secret and computes its SHA-256 hash, called
          the <strong>hashlock</strong>. They lock their coins in the HTLC on their chain
          under that hashlock, with a timeout. The other party sees that lock on-chain, then
          locks their own coins on the other chain under the <strong>same</strong> hashlock,
          with a shorter timeout. Now the initiator claims the second escrow by revealing the
          secret, and that reveal is public on-chain. The counterparty reads the revealed
          secret and uses it to claim the first escrow. One secret unlocks both sides.
        </p>
        <p>
          If anything stalls before the secret is revealed, nothing is lost: each escrow can
          be refunded by its owner once its timeout passes. So a swap either completes on both
          chains or both sides get their money back. The timeouts are deliberately asymmetric:
          the initiator&apos;s window is at least twice the responder&apos;s, so the party who
          reveals the secret always has time to claim before any refund opens against them.
        </p>
        <p>
          Claims are permissionless with the recipient fixed when the escrow is created, so
          anyone can broadcast a claim transaction and the funds still only ever go to the
          chosen recipient.
        </p>
      </LearnSection>

      <LearnSection id="pairs-and-wallets" title="Pairs and the two wallets you need">
        <p>
          The QRL leg of every swap is native Quanta on the QRL 2.0 testnet. The Ethereum leg
          on Sepolia can be one of three assets:
        </p>
        <ul>
          <li>
            <strong>ETH</strong>: the native Sepolia coin.
          </li>
          <li>
            <strong>USDC</strong>: Circle&apos;s official Sepolia test issue.
          </li>
          <li>
            <strong>tUSDT</strong>: a test token that stands in for USDT, deliberately
            reproducing its non-standard ERC-20 behavior.
          </li>
        </ul>
        <p>
          Because the two legs live on two chains, you connect two wallets. For the Sepolia
          leg, QuantaSwap discovers any injected Ethereum wallet through EIP-6963, so MetaMask
          or a similar browser wallet works. For the QRL leg, a picker lists the QRL-capable
          wallets it finds: the QRL browser extension, or MyQRLWallet on mobile or desktop
          paired through a QR code over an encrypted relay, with transactions signed by
          ML-DSA-87 post-quantum keys. If you have never set up a QRL wallet, read{' '}
          <a href="/learn/create-a-wallet">Create a QRL wallet with MyQRLWallet</a> first, and
          see <a href="/learn/connect-dapp-wallet">Connect your wallet to a QRL dApp</a> for
          how QR pairing works. Once connected, your QRL account auto-fills as the recipient
          for the QRL leg.
        </p>
        <p>
          You also need a little gas on each chain you transact on: testnet Quanta from the{' '}
          <Link href="/faucet">ZondScan faucet</Link> (see{' '}
          <a href="/learn/get-testnet-quanta">the faucet guide</a>) and Sepolia ETH from any
          public Sepolia faucet.
        </p>
      </LearnSection>

      <LearnSection id="the-order-book" title="How the order book works">
        <p>
          QuantaSwap runs a maker and taker order book. A <strong>maker</strong> posts an
          offer with an amount and a price; a <strong>taker</strong> accepts it. Each party
          signs two transactions during the swap, and an ERC-20 leg adds a one-time token
          approval transaction, all signed from their own wallets. The
          order book service only coordinates matching. It never holds funds, and both clients
          re-verify every escrow directly on-chain before acting.
        </p>
        <p>
          Makers come in two flavors. A standard listing locks nothing up front: when a taker
          accepts, the maker locks first as the initiator. A pre-funded listing escrows the
          coins on-chain at post time with an open recipient; when a taker accepts, the maker
          assigns that taker as the recipient with a single transaction, and until then the
          maker can release the escrow back to themselves on demand.
        </p>
        <p>
          Makers must be online for a swap to start. The app sends a presence heartbeat while
          the maker&apos;s page is open, and the book marks a maker offline when the heartbeat
          stops. Offline listings are dimmed with a warning that the swap may not start. Open
          listings expire from the book after 48 hours.
        </p>
      </LearnSection>

      <LearnSection id="walk-through-a-swap" title="Walk through a swap as a taker">
        <StepList>
          <Step title="Connect both wallets">
            <p>
              Open <a href="https://quantaswap.io">quantaswap.io</a> and connect an Ethereum
              wallet for the Sepolia leg and a QRL wallet for the QRL leg. Both connections
              show in the header.
            </p>
          </Step>
          <Step title="Take an order">
            <p>
              Pick a pair, then click a row in the book. Each row spells out exactly what you
              send and what you receive before you commit.
            </p>
          </Step>
          <Step title="Wait for the maker to lock or assign">
            <p>
              On a standard order the maker locks their coins first as the initiator. On a
              pre-funded order the escrow already exists and the maker assigns you as its
              recipient. Your client watches the chain and unblocks your next step only after
              it has verified the escrow on-chain: correct hashlock, amount, recipient, asset,
              and timeout.
            </p>
          </Step>
          <Step title="Lock your leg">
            <p>
              You escrow your side under the same hashlock with the shorter timeout. For an
              ERC-20 leg the app first has you approve the HTLC for the exact amount.
            </p>
          </Step>
          <Step title="The maker claims and reveals the secret">
            <p>
              The maker claims your escrow, which publishes the secret on-chain. From this
              moment both legs are claimable with the same secret.
            </p>
          </Step>
          <Step title="Claim with the revealed secret">
            <p>
              Your client reads the now-public secret from the other chain and you claim the
              maker&apos;s escrow with it. The app shows &quot;Atomic swap complete on both
              chains&quot; when both legs settle. Every swap also has a shareable status page,
              and the QRL-leg transactions link straight to ZondScan, so you can follow along
              with <a href="/learn/read-a-transaction">How to read a transaction</a>.
            </p>
          </Step>
        </StepList>
        <Callout type="tip" title="Keep the tab open">
          <p>
            A maker&apos;s browser reports its presence to the order book while the listing
            page stays open; close the tab and the listing soon drops out of the takeable set
            until you return. During an active swap the secret and the swap record live only
            in your browser, so finish the swap in the same browser and avoid clearing site
            data mid-swap: the stored record is what the claim and refund paths need if the
            swap stalls.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="refunds-and-timeouts" title="Refunds and timeouts">
        <p>
          If a swap stalls before the secret is revealed, each locked leg becomes refundable
          to its owner the moment its timeout passes, and a refund button appears in the app.
          In the current testnet build the responder window is about one hour and the
          initiator window about two hours, so a taker waits up to roughly an hour and a maker
          up to roughly two hours to reclaim a stalled leg. Pre-funded listings use a 48-hour
          window sized to the listing lifetime, and their makers can also release an
          unassigned escrow instantly at any time. A taker&apos;s client additionally refuses
          to lock unless the maker&apos;s window leaves at least a 30-minute claim margin
          beyond the taker&apos;s own timeout.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Can the operator or the order book steal my coins?',
            a: (
              <p>
                No. Funds only ever sit in the HTLC contracts, which have no owner, no pause
                switch, and no upgrade path. Escrows pay out either to the recipient the locker fixed
                (at lock time, or at assign time for pre-funded listings) or back to the
                party who locked them, after the timeout or via an early release while a
                pre-funded escrow is still unassigned. The order
                book coordinates matching and holds nothing.
              </p>
            ),
          },
          {
            q: 'What happens if my counterparty disappears mid-swap?',
            a: (
              <p>
                Nothing is lost. Any leg you locked becomes refundable once its timeout
                passes, and until then nobody else can move it. If the secret has already
                been revealed, claim the other leg with it promptly; your client only ever
                locks when the remaining claim window is at least 30 minutes.
              </p>
            ),
          },
          {
            q: 'Is there a trading fee?',
            a: (
              <p>
                The HTLC contract has no fee mechanism. You pay normal network gas for your
                own transactions on each chain, and gas on the QRL testnet is priced in
                Quanta, which the <Link href="/faucet">faucet</Link> hands out for free.
              </p>
            ),
          },
          {
            q: 'Can I practice without a counterparty?',
            a: (
              <p>
                Yes. QuantaSwap ships a sandbox page where one browser plays both roles of a
                swap end to end, which is the fastest way to see all four transactions happen
                before you touch the live book.
              </p>
            ),
          },
          {
            q: 'Where can I see swap transactions on-chain?',
            a: (
              <p>
                The QRL-leg lock and claim are ordinary transactions on the{' '}
                <Link href="/transactions">QRL 2.0 testnet</Link>, and the app links each one to its
                ZondScan page. The Sepolia leg shows up on any Sepolia block explorer.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
