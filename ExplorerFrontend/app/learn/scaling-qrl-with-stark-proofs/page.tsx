import type { Metadata } from 'next';
import { learnMetadata } from '../components/learn-metadata';
import LearnArticle, { LearnSection, Callout, LearnFAQ } from '../components/LearnArticle';

export const metadata: Metadata = learnMetadata('scaling-qrl-with-stark-proofs');

const QUANTASTARK_SITE = 'https://quantastark.com';
const QUANTASTARK_REPO = 'https://github.com/DigitalGuards/QuantaStark';

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="scaling-qrl-with-stark-proofs">
      <p>
        QRL 2.0 signs every transaction with ML-DSA-87, and post-quantum security is paid for in
        bytes: a signature is 4,627 bytes and a public key 2,592 bytes, so every transaction
        carries about 7.2 KB of signature material before its payload. The block gas cap is
        20,000,000 and a plain transfer costs 21,000 gas, so one block holds about 950 transfers.
        Scaling the chain means settling more work per unit of block space while keeping the
        post-quantum guarantees of the base layer intact.
      </p>
      <p>
        This article explains validity proofs in plain English, why the STARK family, whose proofs are
        built from hashes, is the kind of proof the QRL 2.0 virtual machine can check, what the QuantaStark research project
        measured on a local QRL 2.0 network, and what those measurements mean for a layer 2 built
        on QRL.
      </p>
      <Callout type="warning" title="Research status">
        <p>
          QuantaStark is a research project by DigitalGuards, the team behind ZondScan and
          MyQRLWallet. It is experimental, unaudited and subject to breaking changes. Every number
          below was measured on local test networks; nothing is deployed on the public testnet
          yet, and nothing in the project should secure funds.
        </p>
      </Callout>

      <LearnSection id="the-scaling-problem-in-numbers" title="The scaling problem, in numbers">
        <p>
          Two figures fix the throughput of the base layer. The consensus rules cap a block at
          20,000,000 gas, and a native transfer costs 21,000 gas, so a full block carries 952
          transfers and about 6.9 MB of ML-DSA-87 keys and signatures. The second figure is the
          slot time. The Kurtosis test network the project uses as its release gate runs 6-second slots;
          the mainnet parameter set of the consensus client specifies 60-second slots,
          and the public network is expected to follow it. At 60 seconds per slot the base layer
          settles about 16 transfers per second, at 6 seconds about 160.
        </p>
        <p>
          A chain can raise those numbers in two ways. Bigger blocks tend to push out the home
          validators
          that keep the network decentralized. Verifying more work per unit of gas keeps blocks
          small and moves the heavy lifting off chain, and that is what validity proofs do.
        </p>
      </LearnSection>

      <LearnSection id="what-a-validity-proof-is" title="What a validity proof is">
        <p>
          A validity proof turns execution into a claim that can be checked. A prover executes a
          batch of transactions off chain: it checks every signature, applies every balance change
          and computes the new state. It then produces a proof that the batch followed the rules,
          and a verifier contract on the base layer checks that proof. Anyone who trusts the
          verifier contract and the chain it runs on can accept the new state without re-executing
          the batch, and the cost of checking a proof grows slowly with the amount of work it
          attests. The security assumption is the mathematics of the proof system; there is no
          committee to trust and no challenge window to wait out.
        </p>
        <p>
          A validity proof and a zero-knowledge proof are different things, although the two are
          often conflated. A zero-knowledge proof additionally hides the inputs. QuantaStark proves
          correctness and hides nothing: the batch data is published, which is exactly what a
          rollup needs so that anyone can rebuild the state. The project documents that boundary
          explicitly.
        </p>
      </LearnSection>

      <LearnSection id="starks-proofs-built-from-hashes" title="STARKs: proofs built from hashes">
        <p>
          Two families of proof systems are in production on other chains. SNARKs such as Groth16
          and PLONK with KZG commitments rely on elliptic curve pairings. Their proofs are a few
          hundred bytes and cheap to verify, and their security rests on the discrete logarithm
          problem, the assumption Shor&apos;s algorithm breaks. STARKs commit to a computation
          with Merkle trees of hashes and check random openings of those trees with a protocol
          called FRI. Their security rests on hash functions, which Grover&apos;s algorithm only
          weakens, as <a href="/learn/why-post-quantum">why QRL is built for the quantum era</a>{' '}
          explains. A STARK proof is tens of kilobytes, it needs no trusted setup, and it is
          post-quantum by construction.
        </p>
        <p>
          On QRL there is a second, decisive reason to pick STARKs. The QRL 2.0 virtual machine
          has no pairing precompiles. On the 64-byte network the verifier targets, the precompile
          set covers the beacon deposit root, SHA-256,
          ML-DSA-87 signature verification, identity, modular exponentiation and SHAKE256, and
          nothing in it touches an elliptic curve. A pairing-based SNARK verifier cannot run
          there at any gas price. A STARK verifier needs hashing and arithmetic in a finite field, and both are
          cheap on the QRVM: keccak256 is an opcode that costs 30 gas plus 6 per 64-byte word on
          that network.
        </p>
      </LearnSection>

      <LearnSection id="what-quantastark-is" title="What QuantaStark is">
        <p>
          QuantaStark is a verifier for Plonky3-compatible STARK proofs written in Hyperion, the
          smart contract language of QRL 2.0. Plonky3 is the open source proving toolkit under
          several Ethereum zkVMs, including SP1 and OpenVM, so a verifier for its proofs sits on
          the path to general-purpose proving. The repository also holds the Rust prover that
          produces the proofs and test vectors, a fact registry that records verified statements, a
          bridge skeleton whose withdrawals are authorized with ML-DSA-87, and an architecture
          study for a layer 2 derived from the measured numbers.
        </p>
        <p>
          The verifier targets the 64-byte QRL 2.0 network (QIP-55), where the virtual machine
          word is 512 bits, addresses are 64 bytes and every ABI slot is 64 bytes. Existing
          Solidity verifiers served as reference material, and every line of the contract was
          written for the QRVM. Proofs use the Goldilocks field, FRI with keccak256 Merkle trees
          and a keccak-based Fiat-Shamir transcript, the cheapest hashing the machine offers.
        </p>
      </LearnSection>

      <LearnSection id="what-was-measured" title="What was measured">
        <p>
          The benchmark statement is the Fibonacci example that ships with Plonky3, which exercises
          every phase of the verifier. The trace size is the number
          of rows the proof attests. The figures are whole transactions on a gqrl developer node
          at optimizer runs 200 with preset c3: 34 FRI queries, blowup 8 and 16 proof-of-work bits,
          about 118 conjectured bits of security (the provable bound is lower, and the project
          labels every preset experimental).
        </p>
        <div className="overflow-x-auto">
          <table>
            <caption className="sr-only">Measured c3 verification cost by trace size</caption>
            <thead>
              <tr>
                <th scope="col">Trace rows</th>
                <th scope="col">Proof size</th>
                <th scope="col">Transaction gas</th>
                <th scope="col">Inside the verifier</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>
                  4,096 (2<sup>12</sup>)
                </td>
                <td>43,152 bytes</td>
                <td>2,178,874</td>
                <td>1,445,661</td>
              </tr>
              <tr>
                <td>
                  1,048,576 (2<sup>20</sup>)
                </td>
                <td>103,217 bytes</td>
                <td>4,393,186</td>
                <td>2,684,291</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>
          Both cells sit far below the project&apos;s 8,000,000 gas target and the 20,000,000
          block cap. An earlier run of the gas report on a full Kurtosis network with an
          execution client, beacon node and validators matched the developer node cell for cell;
          the cells above are the developer node measurements of 27 August 2026, taken after the
          latest transcript change. The
          verifier compiles to 14,850 bytes of runtime code against the 24,576-byte contract size
          cap. About a third of the transaction cost is calldata, the proof bytes themselves, and
          that share grows with the trace because Merkle authentication paths grow with tree
          height.
        </p>
        <p>
          The binding limit is transaction size. The transaction pool of the execution client
          refuses transactions above 131,072 bytes, so a single proof tops out near 123 KB, which
          the c3 preset reaches somewhere between 2<sup>20</sup> and 2<sup>22</sup> rows. The project&apos;s 8,000,000 gas
          target alone would allow c3 proofs up to 2<sup>28</sup> rows by the model. The study
          prefers two ways past the size cap: recursion, which keeps the final proof small regardless of how much work it attests, and
          staged verification, which splits one proof across several transactions through the fact
          registry.
        </p>
      </LearnSection>

      <LearnSection id="what-this-means-for-a-layer-2" title="What this means for a layer 2">
        <p>
          The architecture study in the repository turns those measurements into a design: a
          validity rollup whose transaction data is posted as calldata on QRL, so anyone can
          rebuild the layer 2 state from the base chain, and whose batches are attested by one
          STARK proof each. The arithmetic per transfer is what makes it interesting.
        </p>
        <p>
          A rollup transfer needs about 16 bytes of calldata: two short account indices, an amount
          and a fee. The 64-byte addresses live in the layer 2 state tree, so they never repeat in
          calldata, and the signature never touches the base layer because the proof attests that
          it was checked. At 16 gas per byte a transfer costs 256 gas of data. A batch made of one
          2<sup>20</sup> proof and two data transactions of about 123 KB each costs about 8.4
          million gas and carries about 15,400 transfers, roughly 540 gas per transfer, about 39
          times below the 21,000 gas of a native transfer. With recursion, where many batch proofs
          are aggregated into one on-chain proof, the cost approaches the calldata floor of about
          260 gas per transfer.
        </p>
        <div className="overflow-x-auto">
          <table>
            <caption className="sr-only">Modelled transfers per slot by configuration</caption>
            <thead>
              <tr>
                <th scope="col">Configuration</th>
                <th scope="col">Transfers per slot</th>
                <th scope="col">At 60 s slots</th>
                <th scope="col">At 6 s slots</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>Native transfers on the base layer</td>
                <td>952</td>
                <td>16 TPS</td>
                <td>159 TPS</td>
              </tr>
              <tr>
                <td>Rollup, one proof and two data transactions per slot</td>
                <td>15,420</td>
                <td>257 TPS</td>
                <td>2,570 TPS</td>
              </tr>
              <tr>
                <td>Rollup, a full block of data (about nine data transactions)</td>
                <td>about 70,000</td>
                <td>about 1,160 TPS</td>
                <td>about 11,600 TPS</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p>
          These are model figures built on the measured verifier cost, and the slot time of the
          public network decides the per-second numbers. One caveat comes with them: a single
          2<sup>20</sup> trace holds about 1,000 to 3,500 signed transfers, so the data-bound rows
          need recursion before they are reachable. Without it the fixed verifier cost dominates
          and a transfer costs 1,500 to 4,200 gas, still 5 to 14 times below the base layer.
        </p>
        <p>
          The study also priced the alternative of keeping data off chain with a committee, a
          validium. Each committee attestation is an ML-DSA-87 signature that costs about 245,000
          gas to verify on the base layer, so a 5-of-7 attestation costs as much as 76 KB of
          calldata. Below about 4,800 transfers per batch the rollup is cheaper on gas as well;
          above that the validium saves gas by adding a trust assumption a post-quantum chain
          should avoid, which is why the study recommends the rollup.
        </p>
      </LearnSection>

      <LearnSection id="what-is-still-open" title="What is still open">
        <p>
          Three problems sit between the measured verifier and a usable layer 2, and the
          project&apos;s roadmap covers all three.
        </p>
        <ul>
          <li>
            <strong>Recursion.</strong> Verifying a STARK inside another STARK is expensive when
            the inner proof is committed with keccak256, because keccak is costly in arithmetic
            circuits. The candidate design uses a circuit-friendly hash for the inner layers and
            keccak256 at the final layer that QRL verifies. That hash is deliberately still open:
            Poseidon2 and Rescue Prime both sit under a security review gate, a QRL core
            contributor recommends Rescue Prime, and cryptanalysis published in 2026 reopened the
            selection.
          </li>
          <li>
            <strong>Signatures on the layer 2.</strong> Verifying ML-DSA-87 inside a proof costs
            one to two orders of magnitude more than a hash-based signature, so the study proposes
            a versioned hash-based scheme for layer 2 accounts, post-quantum by the same argument
            as the STARK itself, and keeps ML-DSA-87 at the bridge. A bridge withdrawal authorized
            with ML-DSA-87 through the precompile measured about 346,000 gas.
          </li>
          <li>
            <strong>Bridge hardening.</strong> The current bridge is a skeleton. Forced inclusion,
            an escape hatch, governance, a public-values schema and an external review of the
            verifier and the circuits come before any deployment that holds value.
          </li>
        </ul>
        <p>
          Nothing here changes the QRL 2.0 protocol. The verifier is an ordinary contract built
          from existing opcodes and precompiles. STARK-friendly precompiles have been discussed in
          the QRL community and would lower the gas further; the design does not depend on them.
        </p>
      </LearnSection>

      <LearnSection id="follow-along" title="Follow along">
        <ul>
          <li>
            <a href={QUANTASTARK_SITE} target="_blank" rel="noopener noreferrer">
              quantastark.com
            </a>
            : the project site with the measured figures and a preview of the testnet bridge,
            enabled once the public 64-byte QRL 2.0 testnet is live.
          </li>
          <li>
            <a href={QUANTASTARK_REPO} target="_blank" rel="noopener noreferrer">
              github.com/DigitalGuards/QuantaStark
            </a>
            : the verifier, the prover, the gas report and the layer 2 architecture study, all
            under GPL-3.0.
          </li>
          <li>
            Once the verifier is deployed on the public testnet, the plan is to publish its contracts
            as verified source on ZondScan, so that every verification transaction can be read
            field by field with <a href="/learn/read-a-transaction">how to read a transaction</a>. The gas
            used on that page is the number this article is about.
          </li>
        </ul>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Is QuantaStark a zero-knowledge system?',
            a: (
              <p>
                No. It produces validity proofs for scaling and publishes the batch data, so the
                proofs hide nothing. Plonky3 has a hiding mode that could add zero knowledge later;
                that would need a new proof layout, verifier support and a fresh security analysis,
                and the project keeps privacy out of scope for the scaling track.
              </p>
            ),
          },
          {
            q: 'Why is a STARK the kind of proof QRL 2.0 can verify?',
            a: (
              <p>
                Pairing-based SNARK verifiers need elliptic curve precompiles, and the QRL 2.0
                virtual machine ships none: its precompiles cover hashing, ML-DSA-87 verification,
                modular exponentiation and identity. A STARK verifier needs hashing and field
                arithmetic, which the machine already does cheaply. The curves behind SNARKs are
                also exactly what a quantum computer breaks, so leaving them out matches the
                chain&apos;s design.
              </p>
            ),
          },
          {
            q: 'Does this change the QRL 2.0 protocol?',
            a: (
              <p>
                No. The verifier is a regular Hyperion contract that runs on the existing opcodes
                and precompiles, and a layer 2 built on it would settle through ordinary
                transactions. Future STARK-friendly precompiles would lower the gas without
                changing the design.
              </p>
            ),
          },
          {
            q: 'When can I use it?',
            a: (
              <p>
                QuantaStark is research software measured on local QRL 2.0 networks. The verifier
                will be deployed on the public 64-byte QRL 2.0 testnet once that network is live,
                and the bridge preview on quantastark.com will be enabled at the same time. Nothing
                in the project should hold real value until it has been reviewed externally.
              </p>
            ),
          },
          {
            q: 'How big is a STARK proof compared with an ML-DSA-87 signature?',
            a: (
              <p>
                A c3 proof for a 4,096-row trace is 43,152 bytes, about nine times a 4,627-byte
                ML-DSA-87 signature, and a proof for a million-row trace is 103,217 bytes. One
                proof attests thousands of operations, which is where the saving comes from.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
