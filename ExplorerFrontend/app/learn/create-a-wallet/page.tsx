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

export const metadata: Metadata = learnMetadata('create-a-wallet');

export default function Page() {
  return (
    <LearnArticle slug="create-a-wallet">
      <p>
        Everything you do on QRL 2.0 starts with a wallet: an address that can
        hold Quanta, sign transactions, and talk to smart contracts. This guide
        walks you through creating one with MyQRLWallet, the web wallet at{' '}
        <a href="https://qrlwallet.com">qrlwallet.com</a>, and backing it up so
        you can open the same account later on desktop or mobile.
      </p>
      <p>
        You will learn what the wallet actually generates behind the scenes,
        why the mnemonic phrase deserves more care than anything else on the
        screen, and how the password-encrypted wallet file fits in. The whole
        process takes about five minutes.
      </p>
      <p>
        QRL 2.0 currently runs as a public testnet. Mainnet arrives after the
        migration from the legacy chain, so the coins you manage with this
        wallet today have no monetary value. That makes right now the ideal
        time to practice: mistakes are free.
      </p>

      <LearnSection id="one-secret-three-backups" title="One secret, three ways to back it up">
        <p>
          When you create an account, the wallet generates a random 48 byte
          seed and attaches a 3 byte descriptor that records the signature
          scheme. Together they form the extended seed, and every key plus the
          Q-prefixed address derive deterministically from it. The wallet
          presents this one secret in three interchangeable formats:
        </p>
        <ul>
          <li>
            <strong>Mnemonic phrase.</strong> 34 words, shown as a numbered
            grid. This is the human-friendly encoding of the extended seed and
            the backup to treat as the master copy.
          </li>
          <li>
            <strong>Hex seed.</strong> The same bytes as a single hexadecimal
            string: <code>0x</code> followed by 102 characters. The wallet
            labels it an alternative method to recover your account.
          </li>
          <li>
            <strong>Encrypted wallet file.</strong> A JSON file named{' '}
            <code>encrypted-wallet-&lt;address&gt;.json</code>, protected by
            the password you set during creation.
          </li>
        </ul>
        <p>
          Any one of the three restores the full account. Lose all three and
          the account is gone permanently: there is no reset flow, no support
          override, and no on-chain recovery.
        </p>
        <Callout type="info" title="Post-quantum keys by default">
          <p>
            Signing keys on QRL 2.0 use ML-DSA-87 (CRYSTALS-Dilithium,
            standardized as NIST FIPS 204), a lattice based scheme built to
            withstand attacks from quantum computers. You get this protection
            automatically; there is nothing to configure. The background is in{' '}
            <a href="/learn/why-post-quantum">why QRL is built for the quantum era</a>.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="create-in-the-browser" title="Create the wallet in your browser">
        <StepList>
          <Step title="Open the web wallet">
            <p>
              Go to <a href="https://qrlwallet.com">https://qrlwallet.com</a>.
              Once the app connects to the network, the &quot;Let&apos;s
              start&quot; card offers to create, import, or connect a wallet.
              Choose <strong>Create a new account</strong>.
            </p>
          </Step>
          <Step title="Set a strong password">
            <p>
              This password encrypts your wallet backup files. It must be at
              least 8 characters and include an uppercase letter, a lowercase
              letter, a number, and a special character. A password manager
              entry is a good home for it.
            </p>
          </Step>
          <Step title="Choose a transaction PIN">
            <p>
              The 4 to 6 digit PIN authorizes day to day transactions, so you
              never retype the seed phrase to send funds. In the browser the
              encrypted seed is erased after 15 minutes of inactivity by
              default, adjustable in Settings. Press{' '}
              <strong>Create account</strong> when both fields validate.
            </p>
          </Step>
          <Step title="Write down the mnemonic offline">
            <p>
              The next screen, titled <strong>Your Recovery Information</strong>,
              shows your new Q address and the 34 word mnemonic. Copy the words
              onto paper in order and store the paper offline. A screenshot or
              a cloud note defeats the purpose of the exercise.
            </p>
          </Step>
          <Step title="Download the encrypted wallet file">
            <p>
              Press <strong>Download Encrypted Wallet File</strong> to save{' '}
              <code>encrypted-wallet-&lt;address&gt;.json</code>. An
              unencrypted variant exists behind an extra warning dialog; skip
              it unless you have a specific reason and a secure destination
              for it.
            </p>
          </Step>
          <Step title="Finish and test the backup">
            <p>
              The <strong>Done</strong> button warns that there is no going
              back once you continue, so confirm your backups first. To prove
              the file works, open <strong>Import an existing account</strong>,
              pick <strong>Import Encrypted Wallet</strong>, select the .json
              file, and enter your password. If the address matches, your
              backup is good.
            </p>
          </Step>
        </StepList>
        <Callout type="warning" title="The mnemonic is the wallet">
          <p>
            Anyone who reads your 34 words controls the account and everything
            in it, from any device, anywhere. And if the words are lost, nobody
            can restore the account: the wallet developers hold no copy and the
            chain has no recovery mechanism. Treat the mnemonic like cash.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="desktop-mobile-extension" title="Desktop, mobile, and the extension">
        <p>MyQRLWallet runs the same wallet on several platforms:</p>
        <ul>
          <li>
            <strong>Web.</strong>{' '}
            <a href="https://qrlwallet.com">qrlwallet.com</a> works in any
            modern browser.
          </li>
          <li>
            <strong>Desktop.</strong> A hardened desktop app for Windows and
            Linux, with releases published at{' '}
            <a href="https://github.com/DigitalGuards/myqrlwallet-desktop">
              github.com/DigitalGuards/myqrlwallet-desktop
            </a>
            . It keeps keys inside an isolated signer process, so during
            creation it shows the mnemonic once and keeps the raw seed out of
            the interface entirely.
          </li>
          <li>
            <strong>Android.</strong> MyQRLWallet is available on Google Play.
          </li>
          <li>
            <strong>iOS.</strong> The iOS app is in beta.
          </li>
          <li>
            <strong>Browser extension.</strong> An extension is in
            preparation.
          </li>
        </ul>
        <p>
          The encrypted wallet file is portable. A file downloaded from the
          web wallet opens in the desktop and mobile apps through the same{' '}
          <strong>Import Encrypted Wallet</strong> screen, so one backup covers
          every device you use.
        </p>
      </LearnSection>

      <LearnSection id="fund-and-explore" title="Fund the wallet and watch it on ZondScan">
        <p>
          A fresh wallet holds 0 Quanta. Request free testnet Quanta from the{' '}
          <Link href="/faucet">ZondScan faucet</Link>; the short guide{' '}
          <a href="/learn/get-testnet-quanta">get testnet Quanta</a> walks
          through it. Once the faucet transaction confirms, paste your Q
          address into the ZondScan search bar to see the balance and history,
          or browse recent activity on the{' '}
          <Link href="/transactions">transactions page</Link>.
        </p>
        <p>
          With a funded wallet you can try the rest of the ecosystem. Start
          with <a href="/learn/connect-dapp-wallet">connecting your wallet to a dApp</a>,
          and keep{' '}
          <a href="/learn/quanta-shor-planck">Quanta, Shor and Planck</a> handy
          for reading amounts along the way.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'How many words does the mnemonic have?',
            a: (
              <p>
                34. They encode the 51 byte extended seed: a 3 byte descriptor
                plus the 48 byte seed itself. All 34 words in the correct order
                are required to restore the account.
              </p>
            ),
          },
          {
            q: 'I lost my password but still have the mnemonic. Am I locked out?',
            a: (
              <p>
                No. The password only protects the wallet file. Import the
                account from the mnemonic, set a new password, and download a
                fresh encrypted wallet file. The old file simply becomes
                undecryptable junk.
              </p>
            ),
          },
          {
            q: 'Is the encrypted wallet file safe to keep in cloud storage?',
            a: (
              <p>
                The file is encrypted with AES-256-GCM, and the key is derived
                from your password with PBKDF2-SHA256 at 600,000 iterations.
                That is strong protection, and it is only as good as the
                password behind it. Keep the file where you would keep other
                sensitive documents, and never upload the unencrypted variant
                anywhere.
              </p>
            ),
          },
          {
            q: 'Can I import my legacy QRL wallet?',
            a: (
              <p>
                No. The legacy QRL chain signs with XMSS, a hash based scheme.
                QRL 2.0 uses ML-DSA-87 with a different seed and address
                format, so a legacy mnemonic cannot be imported. Create a new
                QRL 2.0 wallet for the testnet. The
                migration of legacy balances is part of the mainnet plan; see{' '}
                <a href="/learn/what-is-qrl-2">what is QRL 2.0</a> for how the
                two chains relate.
              </p>
            ),
          },
          {
            q: 'What is the difference between the password and the PIN?',
            a: (
              <p>
                The password encrypts wallet backup files and is what you enter
                when importing a file on a new device. The PIN is a short 4 to
                6 digit code that authorizes transactions during everyday use
                in the web and mobile apps. Both matter; only the mnemonic is
                irreplaceable.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
