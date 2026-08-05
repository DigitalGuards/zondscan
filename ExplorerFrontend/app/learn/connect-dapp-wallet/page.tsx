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

export const metadata: Metadata = learnMetadata('connect-dapp-wallet');

export default function Page() {
  return (
    <LearnArticle slug="connect-dapp-wallet">
      <p>
        Every dApp on QRL 2.0, from the <a href="/learn/stake-with-quantapool">QuantaPool staking pool</a> to
        the <a href="/learn/swap-with-quantaswap">QuantaSwap atomic swap desk</a>, starts with the same step:
        connecting your wallet. Connecting gives the dApp your public Q-address and a way to ask your wallet
        for approvals. Your seed and private keys stay inside the wallet the entire time.
      </p>
      <p>
        This guide explains what a wallet connection actually is, walks through the two ways to connect on
        QRL 2.0 (a browser extension picker and QR pairing with the MyQRLWallet app), and shows how the
        pairing channel is protected with post-quantum encryption. You will finish by pairing your own wallet
        with the live demo dApp hosted here on ZondScan.
      </p>
      <p>
        You need a wallet first: the <a href="/learn/create-a-wallet">create a wallet</a> guide covers that in
        a few minutes. Everything below runs on the public testnet, so the Quanta involved have no monetary
        value and mistakes are free.
      </p>

      <LearnSection id="what-connecting-means" title="What connecting a wallet means">
        <p>
          When you click a dApp&apos;s connect button, the dApp asks your wallet which account it may use. The
          wallet shows you the request, and if you approve, the dApp learns your Q-address. From then on the
          dApp can send further requests: send a transaction, sign a message, sign structured typed data. Each
          one lands in your wallet as an approval prompt, and each one waits for your decision.
        </p>
        <p>
          The dApp receives results only. For a transaction that means a transaction hash it can track on the
          chain, and for a signing request that means an ML-DSA-87 signature plus the matching public key. The
          keys that produce those signatures never leave the wallet. Read-only lookups such as your balance or
          the current block number are proxied through automatically and need no approval at all.
        </p>
      </LearnSection>

      <LearnSection id="two-ways-to-connect" title="The two ways to connect on QRL 2.0">
        <p>
          <strong>Browser extension.</strong> QRL browser extensions such as the MyQRLWallet Extension announce
          themselves to the page through the EIP-6963 discovery standard, so a dApp can show a picker listing
          every wallet it found. You click your extension, an approval popup opens, and once you approve the
          connection is live. Everything stays inside your browser on one machine. If no popup appears, click
          the extension&apos;s icon in the browser toolbar to bring the approval window up.
        </p>
        <p>
          <strong>QR pairing.</strong> This route connects a dApp in one place to a wallet somewhere else,
          typically the MyQRLWallet mobile app on your phone. The dApp generates a one-time
          <code>qrlconnect://</code> URI and displays it as a QR code. You scan it with the mobile app and
          approve the pairing there. On a desktop the same code can instead open the web wallet at{' '}
          <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">qrlwallet.com</a> or the
          desktop app, and there is a copy-code fallback you can paste into the wallet&apos;s dApp connections
          screen. On a phone browser the dApp skips the QR entirely and deep-links straight into the app.
        </p>
      </LearnSection>

      <LearnSection id="an-encrypted-pairing-channel" title="An encrypted pairing channel">
        <p>
          QR pairing needs a relay server to carry messages between the dApp and the wallet, since the two run
          on different devices. Before any request flows, both sides perform a key exchange using ML-KEM-768,
          the NIST-standardized post-quantum key encapsulation mechanism (FIPS 203). Every message after that
          is encrypted with AES-256-GCM. The relay routes ciphertext it cannot read, and each QR code carries a
          fresh single-use pairing secret, so an old code cannot be replayed against a new session.
        </p>
        <p>
          This matches the design philosophy of the chain itself: QRL 2.0 signs transactions with the
          post-quantum ML-DSA-87 scheme, and the connect protocol extends the same threat model to the wallet
          transport. The <a href="/learn/why-post-quantum">why post-quantum</a> article covers the background.
        </p>
        <Callout type="warning" title="Treat pairing codes as secrets">
          <p>
            A pairing QR or connection code is a bearer secret: whoever holds it can complete the pairing.
            Scan codes only from a dApp you opened yourself, and never share a connection code. Also note
            that the dApp name and URL shown in the wallet&apos;s approval screens are supplied by the dApp
            itself and are displayed unverified, so always review what a request actually does before
            approving.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="try-the-qr-flow" title="Try it: pair a wallet with the demo dApp">
        <p>
          The connect SDK ships with a demo dApp, and ZondScan hosts it live. It exercises the full flow:
          pairing, sending a transaction, post-quantum message signing, and read-only RPC calls.
        </p>
        <StepList>
          <Step title="Open the demo dApp">
            <p>
              Go to{' '}
              <a href="https://zondscan.com/dapp-example" target="_blank" rel="noopener noreferrer">
                zondscan.com/dapp-example
              </a>
              . If you want to send a test transaction later, fund your address first at the{' '}
              <Link href="/faucet">ZondScan faucet</Link>; the{' '}
              <a href="/learn/get-testnet-quanta">testnet Quanta guide</a> shows how.
            </p>
          </Step>
          <Step title="Pick MyQRLWallet in the wallet list">
            <p>
              The Connect Wallet card lists every wallet the page discovered. Installed extensions appear under
              their own names, and the entry named MyQRLWallet starts the QR pairing flow. Click it and a QR
              code appears, together with an Open web wallet button, an Open desktop app button, and a Copy
              connection code button.
            </p>
          </Step>
          <Step title="Scan the code with your wallet">
            <p>
              Open MyQRLWallet on your phone, scan the QR code, and confirm the pairing when the app asks. On
              desktop you can instead click Open web wallet, or copy the code and paste it into the
              wallet&apos;s dApp connections screen. The demo&apos;s status line walks through connecting, waiting
              for the wallet, and exchanging keys.
            </p>
          </Step>
          <Step title="Approve account access">
            <p>
              The wallet now shows a connection request naming the dApp. Approving it shares exactly one
              account. The demo switches to Connected and displays your Q-address under Connected Account.
            </p>
          </Step>
          <Step title="Send a request and approve it in the wallet">
            <p>
              Try Sign Message: type any text, click the button, and approve the prompt that appears in your
              wallet. The demo verifies the returned ML-DSA-87 signature locally and prints the result. If you
              grabbed faucet funds, try Send Transaction with a small amount of Quanta, then look the hash up
              under <Link href="/transactions">recent transactions</Link>. The{' '}
              <a href="/learn/read-a-transaction">reading a transaction</a> guide explains every field you will
              see.
            </p>
          </Step>
        </StepList>
      </LearnSection>

      <LearnSection id="approving-and-rejecting-requests" title="Approving and rejecting requests">
        <p>
          Only a small set of methods can reach your approval screen: account access, sending or signing a
          transaction, signing a message, signing typed data, and switching chains. Everything else is either a
          harmless read-only lookup or refused outright by the SDK before it leaves the page. A pairing
          authorizes exactly one account, and signing requests naming any other address fail locally.
        </p>
        <p>
          Rejecting a request is always safe. The dApp receives an error, nothing is signed, and the connection
          itself stays alive for the next request. The mobile wallet additionally requires PIN or biometric
          authentication for every transaction, so an unattended phone cannot approve anything on its own.
        </p>
      </LearnSection>

      <LearnSection id="disconnecting" title="Disconnecting from either side">
        <p>
          Either party can end a session. In the demo dApp the Disconnect button retires the pairing, and a New
          Connection button rotates to a fresh QR code when you want to pair a different wallet. In the wallet,
          the dApp connections screen lists active connections and lets you disconnect any of them. The dApp is
          notified immediately; the demo responds by generating a fresh QR code so you can pair again.
        </p>
        <p>
          Sessions are deliberately durable in between. A paired dApp can restore its session for up to seven
          days without a new scan, and putting the wallet app in the background does not break the pairing: the
          relay buffers an encrypted request for up to five minutes while the wallet is away, and on mobile an
          approval-needing request deep-links the wallet app back to the foreground.
        </p>
      </LearnSection>

      <LearnSection id="for-developers" title="For developers: the connect SDK">
        <p>
          The protocol described above is an open-source TypeScript SDK,{' '}
          <code>@qrlwallet/connect</code> on npm. It exposes a standard EIP-1193 provider, so a dApp talks to a
          remote MyQRLWallet exactly the way it talks to an injected extension:
        </p>
        <CodeBlock
          language="typescript"
          code={`import { QRLConnect } from '@qrlwallet/connect';

const qrl = new QRLConnect({
  dappMetadata: {
    name: 'My QRL dApp',
    url: 'https://mydapp.com',
  },
});

// Render this URI as a QR code, or hand the provider to the
// @qrlwallet/connect-ui pairing modal instead.
const uri = await qrl.getConnectionURI();

// Standard EIP-1193 requests from here on.
const accounts = await qrl.request({ method: 'qrl_requestAccounts' });`}
        />
        <p>
          A sibling package, <code>@qrlwallet/connect-ui</code>, provides a framework-free
          <code>&lt;qrl-pairing-modal&gt;</code> web component that handles the QR rendering, deep link, and
          copy-code fallback for you. Source, protocol documentation, and the demo dApp all live in the{' '}
          <a href="https://github.com/DigitalGuards/myqrlwallet-connect" target="_blank" rel="noopener noreferrer">
            myqrlwallet-connect repository
          </a>
          . The relay is self-hostable if you would rather run your own.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Does a dApp ever see my seed or private keys?',
            a: (
              <p>
                No. Keys stay inside the wallet on both routes. A dApp receives your Q-address, transaction
                hashes, and signatures. Every operation that uses a key runs in the wallet after you approve
                it there.
              </p>
            ),
          },
          {
            q: 'Do I have to scan a QR code every time I visit a dApp?',
            a: (
              <p>
                No. A pairing persists for up to seven days, and the SDK reconnects to it automatically when
                you return to the dApp. A fresh scan is only needed after you disconnect, after the session
                expires, or when you deliberately start a new connection to pair a different wallet.
              </p>
            ),
          },
          {
            q: 'What happens if my phone is locked or the wallet app is closed?',
            a: (
              <p>
                The pairing survives. The relay buffers an encrypted request for up to five minutes while the
                wallet is unreachable, and on mobile an approval-needing request reopens the wallet app through
                a deep link. Opening the app is enough to resume the session.
              </p>
            ),
          },
          {
            q: 'What does rejecting a request do?',
            a: (
              <p>
                The dApp gets an error and nothing is signed. The connection stays active, so you can keep
                using the dApp and approve a later request normally. Rejecting is the right move whenever a
                request looks unexpected.
              </p>
            ),
          },
          {
            q: 'Which dApps can I try this with today?',
            a: (
              <p>
                The demo at{' '}
                <a href="https://zondscan.com/dapp-example" target="_blank" rel="noopener noreferrer">
                  zondscan.com/dapp-example
                </a>{' '}
                is the quickest test. For real testnet flows, both{' '}
                <a href="/learn/stake-with-quantapool">liquid staking with QuantaPool</a> and{' '}
                <a href="/learn/swap-with-quantaswap">atomic swaps with QuantaSwap</a> use the same wallet
                picker and pairing flow described here.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
