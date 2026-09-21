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

export const metadata: Metadata = learnMetadata('launch-a-qrc20');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="launch-a-qrc20">
      <p>
        Launching a token normally means writing a smart contract, setting up a compiler
        toolchain, and deploying by hand. On QRL 2.0 you can skip all of that.{' '}
        <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">
          MyQRLWallet
        </a>{' '}
        includes a built in token creator that deploys a QRC20 token for you through an
        on-chain factory contract. You fill in a form, confirm one transaction, and walk away
        with the contract address of your own token.
      </p>
      <p>
        This tutorial covers the whole path: what a QRC20 token is, what to prepare before you
        start, every field in the creation form, and how to find your new token on ZondScan
        afterward, complete with its holder list and transfer history.
      </p>
      <p>
        Everything here runs on the QRL 2.0 public testnet, which makes it the ideal place to
        practice. Testnet coins have no monetary value, so a mistake costs nothing. The same
        flow will apply on mainnet once the migration from the legacy chain completes.
      </p>

      <LearnSection id="what-is-a-qrc20" title="What is a QRC20 token?">
        <p>
          The QRL 2.0 execution layer is EVM compatible, so it uses the same smart contract
          token model as other EVM chains. A QRC20 token implements the ERC20 interface:
          functions like <code>balanceOf</code>, <code>transfer</code>, <code>approve</code>,
          and <code>totalSupply</code>, plus a <code>Transfer</code> event emitted on every
          movement. On QRL the standard is called QRC20, and ZondScan labels these contracts
          QRC-20.
        </p>
        <p>
          A QRC20 token is itself a smart contract. Your balance is an entry in that
          contract&apos;s storage, and every transfer emits an event that explorers index. That
          shared interface is why any wallet or explorer that understands the standard can
          display your token&apos;s name, symbol, decimals, and balances without special
          support.
        </p>
        <p>
          Smart contracts on QRL 2.0 are written in Hyperion, a close relative of Solidity. If
          you want to write and deploy contracts yourself, the{' '}
          <a href="/learn/deploy-and-verify-contract">deploy and verify guide</a> covers that
          route. This tutorial takes the shortcut: a factory contract that is already deployed
          writes the token for you.
        </p>
      </LearnSection>

      <LearnSection id="before-you-start" title="Before you start">
        <p>You need three things:</p>
        <ul>
          <li>
            <strong>A wallet account with an imported seed.</strong> Token creation signs a
            contract call directly, so the wallet needs access to your signing seed. Accounts
            connected through the browser extension or a paired mobile signer cannot create
            tokens yet; the form tells you so and keeps the submit button disabled. If you are
            new to MyQRLWallet, follow{' '}
            <a href="/learn/create-a-wallet">Create a QRL wallet</a> first.
          </li>
          <li>
            <strong>Testnet Quanta for gas.</strong> Deployment is a normal transaction and
            burns gas like any other. The <Link href="/faucet">ZondScan faucet</Link> funds a fresh
            address in under a minute; see{' '}
            <a href="/learn/get-testnet-quanta">Get testnet Quanta</a> for the walkthrough.
            The factory itself charges no fee, so gas is the entire cost.
          </li>
          <li>
            <strong>A name and symbol.</strong> Something like &quot;My Test Token&quot; with
            the symbol &quot;MTT&quot;. Both fields are required, and the wallet screens them
            for offensive language.
          </li>
        </ul>
      </LearnSection>

      <LearnSection id="create-the-token" title="Create the token step by step">
        <StepList>
          <Step title="Open the token creator">
            <p>
              Sign in at{' '}
              <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">
                qrlwallet.com
              </a>
              , open the Tokens panel, click the plus button, and choose{' '}
              <strong>Create New Token</strong>. A form titled &quot;Create New QRC20
              Token&quot; appears.
            </p>
          </Step>
          <Step title="Enter the name and symbol">
            <p>
              The name is the full label wallets and explorers display. The symbol is the short
              ticker shown next to amounts. Both are fixed at deployment, so check the spelling
              before you continue.
            </p>
          </Step>
          <Step title="Set the initial supply and decimals">
            <p>
              Enter the initial supply in whole tokens; the wallet converts it to base units
              using your decimals setting. Decimals default to 18, matching the convention on
              most EVM chains, and the factory caps them at 18. Unless you have a specific
              reason to change this, leave it at 18.
            </p>
          </Step>
          <Step title="Choose the optional controls">
            <p>Four checkboxes reveal extra fields:</p>
            <ul>
              <li>
                <strong>Mintable.</strong> Reveals a Max Supply field. Leave it unchecked and
                the maximum supply equals your initial supply, which fixes the supply forever.
                Check it and set a higher max, and you, as the token owner, can mint the
                difference later. The maximum supply is immutable once deployed.
              </li>
              <li>
                <strong>Change Initial recipient.</strong> Mints the initial supply to a
                different Q-prefixed address. By default it goes to your own account.
              </li>
              <li>
                <strong>Max Wallet Amount.</strong> A cap on how many tokens a single address
                can hold, checked on every transfer.
              </li>
              <li>
                <strong>Max Transaction Limit.</strong> A cap on how many tokens can move in
                one transfer. Both caps are optional, a value of zero disables them, and the
                owner can adjust them after deployment. The token owner and the initial supply
                recipient are exempt from both caps, and the owner can exempt further addresses
                after deployment.
              </li>
            </ul>
          </Step>
          <Step title="Confirm with your PIN">
            <p>
              Enter your transaction PIN (4 to 6 digits) and press{' '}
              <strong>Create Token</strong>. On the desktop wallet there is no PIN step: the
              isolated signer authorizes the transaction. The wallet switches to a status page
              while your token deploys to the chain.
            </p>
          </Step>
          <Step title="Save the result">
            <p>
              When the transaction confirms, the success card shows the transaction hash, the
              new token&apos;s contract address, its name, symbol, and decimals, the block
              number, and the gas used, denominated in Quanta. The wallet adds the token to
              your token list automatically. Copy the contract address: it is the one
              permanent, unambiguous identifier of your token.
            </p>
          </Step>
        </StepList>
        <p>
          Under the hood the wallet calls <code>createToken</code> on the factory contract.
          The factory deploys a fresh token contract with your parameters, makes your account
          its owner, and emits a <code>TokenCreated</code> event carrying the new address. The
          wallet reads that event from the transaction receipt to show you the result.
        </p>
      </LearnSection>

      <Callout type="warning" title="Permissionless means unvetted">
        <p>
          Token creation is permissionless: anyone can deploy a token with any name and symbol
          in a few minutes, exactly as you just did. A token named after a company, a person,
          or a popular project carries no value, backing, or endorsement because of its name
          alone. Judge every token by its contract address and its on-chain history.
        </p>
      </Callout>

      <LearnSection id="find-your-token-on-zondscan" title="Find your token on ZondScan">
        <p>
          ZondScan indexes new contract deployments and token transfer events as it syncs, so
          your token appears without any registration step. Open the{' '}
          <Link href="/contracts">contracts page</Link> and select the{' '}
          <strong>Tokens (QRC-20)</strong> tab. You can search by token name or contract
          address. Each row shows the token&apos;s name and symbol, contract address, decimals,
          total supply, creator, and the block it was created at.
        </p>
        <p>
          Click through to the token&apos;s own page. The header shows the name, symbol, and
          contract address with copy and QR buttons, alongside stats for total supply,
          decimals, and creator. Two tabs matter most: <strong>Holders</strong> lists every
          address with a balance, and <strong>Transfers</strong> lists every movement of the
          token. Right after creation you will see one holder, the initial recipient, holding
          the full supply.
        </p>
        <p>
          Individual transfers also show up on the transaction pages themselves, decoded with
          the token&apos;s name and amount. The guide on{' '}
          <a href="/learn/read-a-transaction">reading a transaction</a> explains every field,
          including how token transfers differ from plain Quanta transfers.
        </p>
      </LearnSection>

      <LearnSection id="shared-code-and-verification" title="Shared code and verification">
        <p>
          Every token the factory deploys runs the same contract code. The source is published
          under the MIT license in the QRC20-Factory repository on{' '}
          <a href="https://github.com/DigitalGuards" target="_blank" rel="noopener noreferrer">
            GitHub
          </a>
          . The factory stamps out a fresh instance for each token, so two factory tokens can
          differ only in their parameters: name, symbol, supply, decimals, caps, and the
          account that owns them. That predictability is a feature: anyone inspecting a
          factory token knows exactly what it can and cannot do.
        </p>
        <p>
          ZondScan&apos;s contract pages include a code section with the bytecode and, once
          source is published, the verified source and a read/write interface. If you outgrow
          the factory and want custom behavior, write your own contract in Hyperion, deploy
          it, and publish the source through the{' '}
          <Link href="/verify-contract">verification page</Link>. The{' '}
          <a href="/learn/deploy-and-verify-contract">deploy and verify tutorial</a> walks
          through that full workflow.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'What does it cost to create a token?',
            a: (
              <p>
                Gas for the deployment transaction is the entire cost. The factory contract
                charges no fee and the call sends no coins. On the testnet that gas is paid in
                testnet Quanta, which the <Link href="/faucet">faucet</Link> hands out for free.
              </p>
            ),
          },
          {
            q: 'Can I mint more tokens later?',
            a: (
              <p>
                Only if you checked Mintable and set a max supply above the initial supply when
                you created the token. Minting is restricted to the owner and can never push
                the total past the max supply, which is fixed permanently at deployment.
              </p>
            ),
          },
          {
            q: 'Why is the create button disabled for my connected account?',
            a: (
              <p>
                Token creation currently requires an account imported with its seed phrase,
                because the wallet signs the deployment call locally. The wallet marks
                accounts paired through the browser extension or a mobile signer as
                unsupported for now. Import a seed account to create tokens today.
              </p>
            ),
          },
          {
            q: 'Will my token show up on ZondScan automatically?',
            a: (
              <p>
                Yes. The explorer&apos;s synchronizer indexes contract deployments and token
                transfer events as blocks arrive. Your token appears under the Tokens (QRC-20)
                tab on the <Link href="/contracts">contracts page</Link> shortly after the creation
                transaction confirms.
              </p>
            ),
          },
          {
            q: 'Are the wallet and transaction limits permanent?',
            a: (
              <p>
                The owner can change the max wallet amount and max transaction limit at any
                time after deployment, and setting either to zero disables it. The max supply
                is the exception: it is locked in at creation.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
