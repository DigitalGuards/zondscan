import type { Metadata } from 'next';
import Link from 'next/link';
import { learnMetadata } from '../components/learn-metadata';
import LearnArticle, {
  LearnSection,
  StepList,
  Step,
  Callout,
  CodeBlock,
  LearnFAQ,
} from '../components/LearnArticle';

export const metadata: Metadata = learnMetadata('quanta-shor-planck');

const CONVERSION_TABLE = `1 Quanta = 1,000,000,000 Shor            (10^9)
1 Quanta = 1,000,000,000,000,000,000 Planck (10^18)
1 Shor   = 1,000,000,000 Planck           (10^9)
1 Planck = smallest unit, indivisible`;

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="quanta-shor-planck">
      <p>
        Every amount on QRL 2.0, whether it is a wallet balance, a transfer
        value, or a gas fee, lives on chain as a plain integer count of the
        smallest unit. Wallets and explorers translate that integer into
        something a human can read. QRL 2.0 names three units for this
        purpose: <strong>Quanta</strong>, <strong>Shor</strong>, and{' '}
        <strong>Planck</strong>.
      </p>
      <p>
        This article explains what each unit means, gives the exact
        conversion factors, shows which unit each ZondScan page uses, and
        walks through the built in{' '}
        <Link href="/converter">unit converter</Link>. If the chain itself is new
        to you, start with{' '}
        <a href="/learn/what-is-qrl-2">What is QRL 2.0?</a> and come back
        here.
      </p>

      <LearnSection id="the-three-units" title="The three units">
        <ul>
          <li>
            <strong>Quanta</strong> is the whole coin. Balances, transfer
            values, and fees display in Quanta across the QRL 2.0
            ecosystem. QRL remains the name of the project and the exchange
            ticker.
          </li>
          <li>
            <strong>Shor</strong> is one billionth of a Quanta (10^-9). It
            carries over from the legacy QRL chain and sits at a convenient
            scale for gas prices.
          </li>
          <li>
            <strong>Planck</strong> is the base unit: 10^-18 of a Quanta.
            Every value stored on chain is an integer number of Planck. It
            plays the same role that wei plays on Ethereum.
          </li>
        </ul>
        <p>The conversions are exact powers of ten:</p>
        <CodeBlock language="text" code={CONVERSION_TABLE} />
      </LearnSection>

      <LearnSection id="why-two-small-units" title="Why two small units exist">
        <p>
          The legacy QRL chain counts balances in Shor: one coin divides
          into 10^9 Shor, which gives nine decimal places. That precision
          served the legacy chain well for years, and the Shor name was
          already familiar to every QRL holder.
        </p>
        <p>
          QRL 2.0 runs an EVM compatible execution layer, so the entire
          Ethereum toolchain expects the native coin to divide into 10^18
          base units, exactly the way ether divides into wei. To satisfy
          both worlds, QRL 2.0 keeps Shor at its familiar position of
          10^-9 and adds Planck underneath it at 10^-18 as the true base
          unit. Contract math, RPC responses, and raw transaction fields
          all operate on Planck integers; Shor and Quanta exist to make
          those integers readable.
        </p>
        <Callout type="info" title="Raw values are hex Planck">
          When you query a node directly or open the raw view of a
          transaction, amounts appear as hex encoded Planck integers. For
          example, <code>0xde0b6b3a7640000</code> is 10^18 Planck, which
          is exactly 1 Quanta.
        </Callout>
      </LearnSection>

      <LearnSection id="units-on-zondscan" title="How amounts appear on ZondScan">
        <p>
          ZondScan picks the unit that keeps each number readable, so you
          will meet all three while browsing:
        </p>
        <ul>
          <li>
            <strong>Transfer values and fees show in Quanta.</strong> On a
            transaction page the Value row and the Transaction Fee row both
            carry the Quanta label. The{' '}
            <a href="/learn/read-a-transaction">
              guide to reading a transaction
            </a>{' '}
            covers every row in detail, and the live feed is at{' '}
            <Link href="/transactions">Transactions</Link>.
          </li>
          <li>
            <strong>Gas prices show in Shor.</strong> The{' '}
            <Link href="/gas">gas tracker</Link> charts the recent gas price in
            Shor, where typical values land in an easy to read range.
          </li>
          <li>
            <strong>The base fee adapts.</strong> Block pages choose the
            unit by magnitude: values below one Shor display in Planck,
            values below one Quanta display in Shor, and anything larger
            displays in Quanta.
          </li>
          <li>
            <strong>Validator stakes show as whole Quanta.</strong> The
            beacon chain accounts for stake in Shor, and the{' '}
            <Link href="/validators">Validators</Link> page converts those
            balances to whole Quanta for display.
          </li>
        </ul>
      </LearnSection>

      <LearnSection id="use-the-converter" title="Convert units with the built in converter">
        <p>
          For quick math between the three units, ZondScan ships a
          converter that uses exact integer arithmetic, so results are
          never rounded.
        </p>
        <StepList>
          <Step title="Open the converter">
            <p>
              Go to <Link href="/converter">the unit converter</Link>. It works
              entirely in your browser.
            </p>
          </Step>
          <Step title="Enter an amount">
            <p>
              Type a plain decimal number in the Amount field. The result
              updates as you type.
            </p>
          </Step>
          <Step title="Pick the units">
            <p>
              Choose the From and To units from the Quanta, Shor, and
              Planck dropdowns. The swap button between them reverses the
              direction in one click.
            </p>
          </Step>
          <Step title="Read the result">
            <p>
              The converted value appears in the Result field with the
              target unit shown beside it.
            </p>
          </Step>
        </StepList>
        <Callout type="tip" title="The converter refuses impossible fractions">
          Each unit has a hard precision limit: Quanta supports at most 18
          decimal places, Shor supports at most 9, and Planck is
          indivisible. An input with more fractional digits than the
          source unit can represent produces a clear error message, so a
          silent rounding mistake can never reach your copy and paste.
        </Callout>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Is Quanta the same as QRL?',
            a: (
              <p>
                QRL is the project name and the exchange ticker. One coin
                of the native asset is one Quanta, and ZondScan displays
                every native amount in Quanta, Shor, or Planck.
              </p>
            ),
          },
          {
            q: 'What is the smallest amount I can send?',
            a: (
              <p>
                One Planck, which is 10^-18 Quanta. Every on-chain value
                is an integer number of Planck, so there is nothing
                smaller to represent.
              </p>
            ),
          },
          {
            q: 'Which unit do gas prices use?',
            a: (
              <p>
                At the protocol level gas prices are Planck integers. The{' '}
                <Link href="/gas">gas tracker</Link> displays them in Shor
                because typical prices sit in a readable range there.
              </p>
            ),
          },
          {
            q: 'Does the legacy chain use these units too?',
            a: (
              <p>
                The legacy QRL chain uses Shor as its smallest unit, with
                10^9 Shor per coin. QRL 2.0 keeps Shor at that same
                position and adds Planck below it so the chain can offer
                the 18 decimals that EVM tooling expects.
              </p>
            ),
          },
          {
            q: 'Do testnet amounts have monetary value?',
            a: (
              <p>
                No. QRL 2.0 currently runs as a public testnet, and
                testnet coins have no monetary value. You can request free
                testnet Quanta from the <Link href="/faucet">faucet</Link>; the{' '}
                <a href="/learn/get-testnet-quanta">faucet guide</a> walks
                through it step by step.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
