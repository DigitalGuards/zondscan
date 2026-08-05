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

export const metadata: Metadata = learnMetadata('get-testnet-quanta');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="get-testnet-quanta">
      <p>
        Every hands-on tutorial on the QRL 2.0 testnet starts with the same requirement: an
        address that holds some testnet Quanta to pay for gas. ZondScan runs a built in faucet
        at <Link href="/faucet">/faucet</Link> that sends a small amount of testnet Quanta to any
        valid address, free of charge.
      </p>
      <p>
        This guide covers what the faucet sends and how often you can claim, then walks through
        a claim step by step: from copying your address to watching the transaction confirm on
        the explorer. If QRL 2.0 itself is new to you, read{' '}
        <a href="/learn/what-is-qrl-2">What is QRL 2.0?</a> first.
      </p>
      <p>
        The faucet exists for developers and testers getting ready ahead of mainnet. With a
        funded testnet address you can send transfers, deploy contracts, and try out wallets
        and dApps without spending anything real.
      </p>

      <Callout type="warning" title="Testnet Quanta has no monetary value">
        <p>
          Testnet coins cannot be bought or sold, and they carry no value outside the test
          network. They exist purely so you can experiment. Anyone offering to sell you testnet
          Quanta is running a scam.
        </p>
      </Callout>

      <LearnSection id="what-the-faucet-sends" title="What the faucet sends">
        <p>
          At the time of writing the faucet sends <strong>10 Quanta</strong> per claim by
          default. These are operator settings and can change, so treat the faucet page itself
          as the source of truth: it always displays the live drip amount and cooldown at the
          top of the form. If 10 Quanta sounds abstract, the{' '}
          <a href="/learn/quanta-shor-planck">units article</a> and the{' '}
          <Link href="/converter">converter</Link> break down how Quanta relates to Shor and Planck.
        </p>
        <p>Three limits keep the faucet from being drained:</p>
        <ul>
          <li>
            <strong>Per address cooldown.</strong> Each address can claim once per cooldown
            window, 24 hours by default.
          </li>
          <li>
            <strong>Per IP cooldown.</strong> The same window applies to your network
            connection. Claiming again with a fresh address from the same connection still hits
            the cooldown.
          </li>
          <li>
            <strong>Daily budget.</strong> The faucet can enforce a global cap on the total it
            pays out across all users in a rolling 24 hour period. Once the budget is spent,
            claims resume the next day.
          </li>
        </ul>
        <p>
          Every claim is also protected by a Cloudflare Turnstile captcha. The submit button
          stays disabled until you complete the challenge, and each solved challenge is valid
          for exactly one claim.
        </p>
      </LearnSection>

      <LearnSection id="claim-step-by-step" title="Claim testnet Quanta step by step">
        <StepList>
          <Step title="Create or open a wallet and copy your address">
            <p>
              You need a Q-prefixed QRL 2.0 address to receive the coins. If you do not have
              one yet, follow <a href="/learn/create-a-wallet">Create a QRL wallet</a> to set
              one up in a few minutes with{' '}
              <a href="https://qrlwallet.com">MyQRLWallet</a>. Then copy your address: it
              starts with <code>Q</code> followed by 40 hexadecimal characters.
            </p>
          </Step>
          <Step title="Open the faucet">
            <p>
              Go to <Link href="/faucet">/faucet</Link>. The page shows the current drip amount and
              cooldown. If the faucet is temporarily offline the page replaces the form with a
              notice; check back later.
            </p>
          </Step>
          <Step title="Paste your address, solve the captcha, and submit">
            <p>
              Paste the full <code>Q&hellip;</code> address into the input, complete the
              Turnstile challenge, and press the request button. The form checks the address
              format before sending anything, so a typo or a <code>0x</code>-prefixed value is
              caught immediately with a pointer to the correct Q-prefixed format.
            </p>
          </Step>
          <Step title="Follow the transaction link">
            <p>
              On success the page shows the amount broadcast and a transaction hash that links
              straight to the transaction page. The transfer starts out pending and normally
              confirms within a minute. While you wait, see{' '}
              <a href="/learn/read-a-transaction">How to read a transaction</a> to make sense
              of every field on that page.
            </p>
          </Step>
          <Step title="Check the balance on your address page">
            <p>
              Paste your address into the ZondScan search bar, or open{' '}
              <code>/address/&lt;your address&gt;</code> directly. Once the transaction
              confirms, your balance updates and the faucet transfer appears in the
              address&apos;s transaction list. You are ready for the rest of the tutorials.
            </p>
          </Step>
        </StepList>
      </LearnSection>

      <LearnSection id="when-a-claim-fails" title="When a claim fails">
        <p>The faucet tells you exactly why a claim was refused. The common cases:</p>
        <ul>
          <li>
            <strong>&quot;You have already claimed recently&quot;</strong> means the address or
            IP cooldown is still active. The error includes an approximate wait time; try again
            after it passes.
          </li>
          <li>
            <strong>Invalid address</strong> means the input does not match the expected
            format. QRL 2.0 addresses start with <code>Q</code>; the <code>0x</code> prefix on
            this network belongs to block and transaction hashes.
          </li>
          <li>
            <strong>Captcha verification failed</strong> usually means the challenge token
            expired before you submitted. Solve it again and resubmit; each token is single
            use.
          </li>
          <li>
            <strong>Daily limit reached</strong> means the global budget for the rolling 24
            hour window is spent. Claims open up again the next day.
          </li>
          <li>
            <strong>Out of funds or offline</strong> means the faucet account itself needs a
            top up or the service is down for maintenance. Both are temporary.
          </li>
        </ul>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'How much does the faucet send per claim?',
            a: (
              <p>
                The default is 10 Quanta per claim, once per address per 24 hours. Both numbers
                are operator settings and can change; the live values are always displayed on
                the <Link href="/faucet">faucet page</Link> itself.
              </p>
            ),
          },
          {
            q: 'Can I claim for several addresses in one day?',
            a: (
              <p>
                Generally no. The cooldown applies per address and per IP, so a second claim
                from the same connection waits for the same window even if it targets a
                different address. If you need more testnet Quanta for a project, spread your
                claims across days or ask in the community channels.
              </p>
            ),
          },
          {
            q: 'My transaction is still pending. Is something wrong?',
            a: (
              <p>
                Usually not. The faucet responds as soon as the node accepts the broadcast, and
                the transfer then waits in the pending pool until a block includes it, normally
                within a minute. You can watch it on the{' '}
                <Link href="/pending/1">pending transactions</Link> page or refresh the transaction
                page.
              </p>
            ),
          },
          {
            q: 'Where do the faucet coins come from?',
            a: (
              <p>
                From a funded testnet account operated alongside the explorer. Each drip is an
                ordinary native transfer signed with a post-quantum ML-DSA-87 signature, so you
                can inspect it on ZondScan exactly like any other transaction.
              </p>
            ),
          },
          {
            q: 'Will my testnet Quanta carry over to mainnet?',
            a: (
              <p>
                No. Testnet balances exist only on the test network and can disappear whenever
                the network resets. Mainnet arrives later, after the migration from the legacy
                chain, with its own genesis state.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
