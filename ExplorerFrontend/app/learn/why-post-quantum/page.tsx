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

export const metadata: Metadata = learnMetadata('why-post-quantum');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="why-post-quantum">
      <p>
        Every blockchain transaction is authorized by a digital signature. Whoever can produce a
        valid signature for an account controls its funds, so the security of the whole ledger
        reduces to the security of one cryptographic primitive. Bitcoin and Ethereum both rely on
        signatures over the secp256k1 elliptic curve for that primitive, and it is exactly the
        piece a large quantum computer would break.
      </p>
      <p>
        This article explains how signatures secure blockchains today, what Shor&apos;s and
        Grover&apos;s algorithms actually threaten, why the phrase &quot;harvest now, decrypt
        later&quot; matters more for a public ledger than for almost any other system, and which
        post-quantum standards NIST finalized in 2024. It then covers how QRL answered: a legacy
        chain built on hash based XMSS since 2018, and{' '}
        <a href="/learn/what-is-qrl-2">QRL 2.0</a> built on lattice based ML-DSA-87.
      </p>
      <p>
        The tone here is deliberately measured. Nobody can say when a cryptographically relevant
        quantum computer will exist, and this article makes no prediction. The case for
        post-quantum signatures rests on a property of blockchains themselves: everything written
        to the ledger stays public forever.
      </p>

      <LearnSection id="how-signatures-secure-blockchains" title="How signatures secure blockchains today">
        <p>
          A blockchain account is a keypair. The private key stays with the owner; the public key,
          or a hash of it, becomes the address other people send funds to. To spend, the owner
          signs the transaction with the private key, and every node verifies that signature
          against the public key before accepting the transaction into a block.
        </p>
        <p>
          Bitcoin and Ethereum both build their signatures on the secp256k1 curve: Ethereum uses
          ECDSA everywhere, and Bitcoin uses ECDSA plus Schnorr signatures since the Taproot
          upgrade. Their security rests on the
          elliptic curve discrete logarithm problem: given a public key, deriving the private key
          behind it is believed to take a classical computer on the order of{' '}
          <code>2^128</code> operations. That number is so far beyond any realistic amount of
          computation that the scheme is considered secure against every classical attacker.
        </p>
        <p>
          The important word is classical. The hardness of the discrete logarithm problem is an
          assumption about the machines doing the attacking, and quantum computers change the
          machines.
        </p>
      </LearnSection>

      <LearnSection id="shors-algorithm" title="Shor breaks the curve">
        <p>
          In 1994 Peter Shor published a quantum algorithm that solves integer factoring and
          discrete logarithms in polynomial time. Run on a large, fault-tolerant quantum computer,
          it turns the problem underneath ECDSA from infeasible into routine: given a secp256k1
          public key, such a machine could compute the matching private key and sign anything on
          the owner&apos;s behalf. The same algorithm breaks RSA and classical Diffie-Hellman.
        </p>
        <p>
          Running Shor at cryptographic scale requires a few thousand error-corrected logical
          qubits, which with current error-correction overheads means millions of physical
          qubits. Publicly known hardware is orders of magnitude away from that. The threat model
          for a blockchain still has to take it seriously, because a ledger written today is
          expected to remain secure for decades, and its entire history is available to any future
          attacker.
        </p>
      </LearnSection>

      <LearnSection id="grover-and-hash-functions" title="Grover only dents hash functions">
        <p>
          The second relevant quantum algorithm is Grover&apos;s search, published in 1996. It
          speeds up brute-force search quadratically: finding a preimage for an n-bit hash takes
          about <code>2^(n/2)</code> quantum operations instead of <code>2^n</code>. In effect it
          halves the security level of a hash function.
        </p>
        <p>
          Halving is survivable. SHA-256 retains roughly 128 bits of preimage resistance against a
          quantum attacker, which remains far out of reach, and doubling output sizes restores the
          original margin wherever more is needed. This asymmetry is the foundation of post-quantum
          cryptography: structured problems like discrete logarithms collapse under Shor, while
          hash functions and certain lattice problems stand with adjusted parameters. Hash based
          signature schemes such as XMSS inherit their security directly from the hash function,
          which is why they are considered quantum resistant.
        </p>
      </LearnSection>

      <LearnSection id="harvest-now-decrypt-later" title="Harvest now, decrypt later">
        <p>
          &quot;Harvest now, decrypt later&quot; describes an attacker who records protected data
          today and waits for the hardware to attack it. For encrypted network traffic the attacker
          at least has to capture the ciphertext in transit. A blockchain removes even that step:
          the ledger is a permanent, public, perfectly ordered archive that anyone can download.
        </p>
        <p>
          What matters is when the public key itself becomes visible. Legacy and SegWit Bitcoin
          address types publish only a hash of the public key until the first spend reveals it,
          and any address that is reused after spending sits on-chain with its public key exposed.
          Taproot addresses expose a public key as soon as they receive funds.
          Account based chains like Ethereum reveal the public key with the first outgoing
          transaction, and accounts are reused by design, so essentially every active account has
          an exposed key. A future attacker running Shor&apos;s algorithm over that archive could
          derive private keys and produce valid signatures for those accounts.
        </p>
        <Callout type="info" title="The measured takeaway">
          <p>
            The concern is structural. Signatures published today must stay unforgeable for as
            long as the funds behind them exist, so the honest question for a ledger designer is
            whether an assumption known to fail on quantum hardware belongs in that role at all.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="the-nist-standards" title="The NIST standards of 2024">
        <p>
          Post-quantum cryptography stopped being exotic in August 2024, when NIST finalized three
          standards after an eight-year public competition:
        </p>
        <ul>
          <li>
            <strong>FIPS 203, ML-KEM</strong>: a module lattice key encapsulation mechanism (from
            the scheme formerly known as Kyber), used to establish shared secrets for encryption.
          </li>
          <li>
            <strong>FIPS 204, ML-DSA</strong>: a module lattice digital signature algorithm, from
            the CRYSTALS-Dilithium family.
          </li>
          <li>
            <strong>FIPS 205, SLH-DSA</strong>: a stateless hash based signature scheme, the
            conservative option whose security rests only on hash functions.
          </li>
        </ul>
        <p>
          These standards give implementers vetted algorithms with fixed, reviewed parameter sets.
          A chain adopting them today builds on the same primitives governments and browsers are
          migrating to.
        </p>
      </LearnSection>

      <LearnSection id="how-qrl-answers" title="How QRL answers, twice">
        <p>
          QRL has treated the quantum threat as a genesis-day requirement since 2018. The legacy
          QRL chain launched with XMSS (RFC 8391), a hash based signature scheme. Its security
          reduces to the hash function, which Grover only weakens, so it is quantum resistant by
          construction. XMSS is stateful: each key is a Merkle tree of one-time signatures, every
          signature consumes one leaf index, and reusing an index is catastrophic, so the wallet
          must track which indexes have been spent.
        </p>
        <p>
          <a href="/learn/what-is-qrl-2">QRL 2.0</a> moves to ML-DSA-87, the strongest parameter
          set of FIPS 204, at NIST security level 5. It is lattice based and stateless: one account
          can sign an unbounded number of transactions with no index bookkeeping, which is what an
          EVM style account model needs. Every Q-prefixed account and every transaction on the
          chain is signed with ML-DSA-87 from genesis, so the ledger never accumulates a backlog of
          quantum-vulnerable ECDSA signatures the way an existing chain would during a migration.
          Consensus is proof of stake on a beacon chain, covered in{' '}
          <a href="/learn/validators-and-epochs">validators and epochs</a>.
        </p>
        <p>
          For an outside view of where the wider industry stands, the independent{' '}
          <a href="https://qrindex.org/" target="_blank" rel="noopener noreferrer">
            Blockchain Quantum Readiness Index
          </a>{' '}
          scores over a hundred chains on quantum readiness using a published methodology, and
          places QRL among the handful of projects in its top quantum-ready tier.
        </p>
        <Callout type="warning" title="Testnet status">
          <p>
            QRL 2.0 currently runs as a public testnet, with mainnet planned after the migration
            from the legacy chain. Testnet Quanta have no monetary value, which makes now a safe
            time to try everything below.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="beyond-signatures" title="Post-quantum beyond signatures">
        <p>
          Signatures are the piece the chain itself depends on, and the surrounding tooling follows
          the same policy. The MyQRLWallet dApp pairing protocol establishes its sessions with
          ML-KEM-768, the FIPS 203 key encapsulation mechanism, and encrypts the channel with
          AES-256-GCM. Pairing a wallet to a dApp is covered in{' '}
          <a href="/learn/connect-dapp-wallet">connecting your wallet to a dApp</a>.
        </p>
      </LearnSection>

      <LearnSection id="see-it-on-the-testnet" title="See it on the testnet">
        <p>
          The fastest way to make this concrete is to put a post-quantum signature on-chain
          yourself and inspect it.
        </p>
        <StepList>
          <Step title="Create a wallet">
            <p>
              Follow <a href="/learn/create-a-wallet">the wallet tutorial</a> at{' '}
              <a href="https://qrlwallet.com" target="_blank" rel="noopener noreferrer">
                qrlwallet.com
              </a>{' '}
              and note your Q-prefixed address.
            </p>
          </Step>
          <Step title="Fund it with testnet Quanta">
            <p>
              Use the <Link href="/faucet">ZondScan faucet</Link>, or read{' '}
              <a href="/learn/get-testnet-quanta">the faucet guide</a> first.
            </p>
          </Step>
          <Step title="Send a transaction and inspect it">
            <p>
              Find it under <Link href="/transactions">recent transactions</Link> and decode the fields
              with <a href="/learn/read-a-transaction">how to read a transaction</a>. Every
              confirmed transaction you see was authorized by an ML-DSA-87 signature.
            </p>
          </Step>
          <Step title="Watch consensus happen">
            <p>
              Browse the <Link href="/validators">validator set</Link> to see the proof of stake side of
              the chain at work.
            </p>
          </Step>
        </StepList>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Can any quantum computer today break ECDSA?',
            a: (
              <p>
                No publicly known machine comes close. Running Shor&apos;s algorithm against a
                256-bit curve requires thousands of error-corrected logical qubits, built from
                millions of physical qubits, and remains far beyond current hardware. The
                argument for acting early rests on the permanence of the ledger:
                public keys exposed on-chain today remain attackable by whatever hardware exists in
                the future.
              </p>
            ),
          },
          {
            q: 'Why did QRL 2.0 adopt ML-DSA-87 when the legacy chain already had XMSS?',
            a: (
              <p>
                XMSS is stateful: every key holds a finite pool of one-time signature indexes, and
                the wallet must never reuse one. ML-DSA-87 is stateless, so a single account can
                sign any number of transactions, which fits the EVM account model of QRL 2.0. Its
                parameters sit at NIST security level 5, the highest category FIPS 204 defines.
              </p>
            ),
          },
          {
            q: 'Are post-quantum signatures larger than ECDSA signatures?',
            a: (
              <p>
                Yes, substantially. An ECDSA signature is roughly 64 to 72 bytes, while an
                ML-DSA-87 signature is 4,627 bytes with a 2,592 byte public key. QRL 2.0 uses
                these signatures natively from genesis, so the protocol and its tooling are built
                around those sizes.
              </p>
            ),
          },
          {
            q: 'Do quantum computers break SHA-256?',
            a: (
              <p>
                Grover&apos;s algorithm halves its effective preimage resistance from 256 to about
                128 bits, which is still far beyond reach. Hash functions survive the quantum
                transition with adequate parameters, which is exactly why hash based schemes like
                XMSS and SLH-DSA are trusted as post-quantum signatures.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
