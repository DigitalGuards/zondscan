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

export const metadata: Metadata = learnMetadata('read-a-transaction');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="read-a-transaction">
      <p>
        Every action on QRL 2.0, from a simple transfer to a contract
        deployment, is recorded as a transaction. The ZondScan transaction page
        collects everything the network knows about one of them: who sent it,
        where it went, what it cost, and what it changed.
      </p>
      <p>
        This guide walks through the page field by field. You will learn how to
        find a transaction, how to read its status and confirmation count, how
        value and fees are denominated, and what the decoded panels for token
        transfers, event logs, and internal calls are telling you.
      </p>
      <p>
        Everything here applies to the current QRL 2.0 public testnet, where
        coins have no monetary value. That makes it a safe place to practice:{' '}
        <a href="/learn/get-testnet-quanta">grab some testnet Quanta from the
        faucet</a>, send yourself a transfer, and follow along on a real page.
      </p>

      <LearnSection id="find-a-transaction" title="Find the transaction">
        <p>There are three common ways to land on a transaction page.</p>
        <StepList>
          <Step title="Search for it directly">
            <p>
              Paste the transaction hash into the search bar at the top of any
              ZondScan page. The search accepts addresses, transaction hashes,
              and block numbers. A transaction hash starts with{' '}
              <code>0x</code> followed by 64 hexadecimal characters.
            </p>
          </Step>
          <Step title="Browse from an address page">
            <p>
              If you know the sender or recipient, open their address page and
              use the <strong>Transactions</strong> tab. The{' '}
              <strong>Internal Txns</strong> tab lists calls that reached the
              address from inside contract execution.
            </p>
          </Step>
          <Step title="Browse recent network activity">
            <p>
              The <Link href="/transactions">transactions list</Link> shows recently
              confirmed transactions across the whole chain, and the{' '}
              <Link href="/pending/1">pending list</Link> shows what is currently
              waiting in the mempool.
            </p>
          </Step>
        </StepList>
      </LearnSection>

      <LearnSection id="status-and-confirmations" title="Status and confirmations">
        <p>
          The badge at the top of the page gives you the verdict at a glance.
          You will see one of three states:
        </p>
        <ul>
          <li>
            <strong>Confirmed</strong>: the transaction is included in a block
            and executed successfully.
          </li>
          <li>
            <strong>Pending</strong>: the transaction is known to the network
            and has no block yet.
          </li>
          <li>
            <strong>Reverted</strong>: the transaction made it into a block,
            then the EVM rolled it back. Any state changes it attempted were
            discarded. The gas consumed by the partial execution is still paid,
            so a reverted transaction has a real fee.
          </li>
        </ul>
        <p>
          Next to the status badge is the confirmation count. A transaction has
          one confirmation the moment it lands in a block, and the count grows
          by one with every block produced after it. For recent transactions
          the page updates the count live, stopping once it passes a couple
          dozen. The <strong>Block</strong> row links to the block that
          included the transaction, and the <strong>Timestamp</strong> row
          shows when that block was produced, in UTC.
        </p>
      </LearnSection>

      <LearnSection id="from-to-and-value" title="From, to, and value">
        <p>
          <strong>From</strong> is the Q-prefixed address that signed the
          transaction, and <strong>To</strong> is its target. Both link to the
          corresponding address pages and have copy buttons.
        </p>
        <p>
          <strong>Value</strong> is the amount of native coin the transaction
          moved, displayed in Quanta. One Quanta is 10^9 Shor and 10^18 Planck;
          the chain stores the raw Planck integer and the explorer converts it
          for display. The <Link href="/converter">unit converter</Link> does exact
          math between all three units, and{' '}
          <a href="/learn/quanta-shor-planck">the units article</a> explains
          where each one is used.
        </p>
      </LearnSection>

      <LearnSection id="gas-and-the-fee" title="Gas and the transaction fee">
        <p>
          QRL 2.0 uses the EVM gas model. Three numbers determine what a
          transaction costs:
        </p>
        <ul>
          <li>
            <strong>Gas limit</strong>: the maximum amount of computation the
            sender authorized. While a transaction is pending, ZondScan shows
            this as its own row.
          </li>
          <li>
            <strong>Gas used</strong>: how much of that budget execution
            actually consumed. A plain transfer uses a small fixed amount;
            contract calls use more.
          </li>
          <li>
            <strong>Gas price</strong>: what the sender offered to pay per unit
            of gas, quoted in Shor.
          </li>
        </ul>
        <p>
          On the confirmed transaction page these fold into the{' '}
          <strong>Transaction Fee</strong> row: gas used multiplied by gas
          price, displayed in Quanta. When a market price is available the page
          adds an approximate USD equivalent next to it. For a live view of
          what the network is currently charging, the{' '}
          <Link href="/gas">Gas Tracker</Link> summarizes recent gas prices in Shor
          and the estimated cost of a simple transfer.
        </p>
      </LearnSection>

      <LearnSection id="token-transfers" title="Decoded token transfers">
        <p>
          When a transaction moves tokens, ZondScan decodes the transfer events
          and renders one card per transfer. For a QRC-20 transfer the card
          shows the token name and symbol linked to the token contract, the
          amount formatted with the token&apos;s own decimals, and the sender
          and recipient. NFT transfers get the same treatment with a{' '}
          <strong>Token ID</strong> row, labeled QRC-721 for single tokens and
          QRC-1155 for multi-token contracts. A single transaction can emit
          several transfer events, for example a swap, so cards can stack.
        </p>
        <p>
          Two badges are worth knowing. <strong>Mint</strong> means the
          transfer came from the zero address, so new tokens were created.{' '}
          <strong>Burn</strong> means the transfer went to the zero address, so
          tokens were destroyed.
        </p>
        <p>
          Below the transfer cards, the <strong>Event Logs</strong> panel lists
          every log the transaction emitted. Well-known token events are
          decoded inline, and events from verified contracts are decoded
          through their published ABI with labeled arguments. Anything else is
          shown as raw topics and data you can copy into an external decoder.
        </p>
        <Callout type="tip" title="Raw hex means the contract is unverified">
          If the Event Logs or Input Data panels show undecoded hex, the
          contract behind them has no verified source on ZondScan. Anyone with
          the source can fix that through{' '}
          <Link href="/verify-contract">contract verification</Link>; after that,
          every visitor sees named events and labeled arguments on this
          transaction.
        </Callout>
      </LearnSection>

      <LearnSection id="internal-transactions" title="Internal transactions">
        <p>
          A transaction targeting a contract can trigger further calls while it
          executes, and some of those calls move coins of their own. ZondScan
          records the nested call frames that moved native value, plus any
          contract creations, as internal transactions; they appear in the{' '}
          <strong>Internal Transactions</strong> panel, one row per call frame,
          with the call type, the from and to addresses, the gas the frame
          used, and the native value it carried. Nested calls are indented to
          show depth.
        </p>
        <p>
          The panel only appears when there is something to show; simple
          transfers have no internal transactions. The same rows also surface
          on the affected address pages under the Internal Txns tab.
        </p>
      </LearnSection>

      <LearnSection id="pending-and-eta" title="Pending transactions and the inclusion ETA">
        <p>
          Before a transaction is mined it sits in the mempool, and ZondScan
          gives it a dedicated live view. The{' '}
          <Link href="/pending/1">pending transactions list</Link> refreshes every
          five seconds and marks each entry pending, mined, or dropped. Opening
          a pending transaction shows how long it has been waiting, an
          estimated time to inclusion, the current mempool depth, and whether
          its gas price sits above or below the mempool median.
        </p>
        <p>
          The pending page decodes token transfer calldata before confirmation,
          so you can already see what a QRC-20 transfer intends to do. Once the
          transaction is included in a block, the page navigates to the
          confirmed view on its own. If it disappears from the mempool without
          being mined, the page reports it as dropped or replaced; the usual
          cause is a second transaction with the same nonce and a higher gas
          price.
        </p>
      </LearnSection>

      <LearnSection id="contract-deployments" title="What a contract deployment looks like">
        <p>
          A contract deployment is a transaction with no recipient. The sender
          publishes bytecode, and the network derives a fresh address for the
          new contract. On ZondScan the <strong>To</strong> row shows a{' '}
          <strong>Contract Created</strong> label with the new contract&apos;s
          address, and a dedicated Contract Created card repeats the address
          with more detail. When the deployed contract is a token, the card
          adds a standard badge (QRC-20, QRC-721, or QRC-1155) and the token
          name and symbol.
        </p>
        <p>
          If you want to produce one of these yourself, the{' '}
          <a href="/learn/deploy-and-verify-contract">deploy and verify
          tutorial</a> walks through compiling, deploying to the testnet, and
          publishing verified source. Deployed contracts are browsable on the{' '}
          <Link href="/contracts">contracts page</Link>.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Why does my transaction show Reverted?',
            a: (
              <p>
                It was included in a block and its execution failed, so the
                EVM discarded every state change it attempted. Common causes
                are a failed require check, an insufficient token balance, or
                running out of gas. The fee for the gas consumed up to the
                failure point is still paid.
              </p>
            ),
          },
          {
            q: 'How many confirmations should I wait for?',
            a: (
              <p>
                Each confirmation is one more block built on top of the one
                containing your transaction, so deeper means harder to reorder.
                ZondScan stops refining the live count after a couple dozen
                confirmations, the range most chain interfaces treat as
                effectively final.
              </p>
            ),
          },
          {
            q: 'Why does the To field show a Contract Created label?',
            a: (
              <p>
                That transaction deployed a contract. Deployments carry no
                recipient; the network assigns the new contract an address when
                the transaction executes, and ZondScan links that address in
                place of the empty field.
              </p>
            ),
          },
          {
            q: 'What do the Mint and Burn badges on a token transfer mean?',
            a: (
              <p>
                Token standards represent creation and destruction as transfers
                involving the zero address. A transfer from the zero address
                minted new tokens, and a transfer to the zero address burned
                them, so ZondScan replaces the address with a badge.
              </p>
            ),
          },
          {
            q: 'Why does the Input Data panel show raw hex?',
            a: (
              <p>
                The calldata matched none of the well-known token methods and
                the target contract has no verified ABI on ZondScan. Verifying
                the contract on the{' '}
                <Link href="/verify-contract">verify contract page</Link> lets the
                explorer decode the method name and arguments for everyone.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
