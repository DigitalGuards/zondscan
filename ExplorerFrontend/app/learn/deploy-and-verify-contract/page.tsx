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

export const metadata: Metadata = learnMetadata('deploy-and-verify-contract');

const contractSource = `// SPDX-License-Identifier: MIT
pragma hyperion ^0.0.2;

contract Counter {
    uint256 public count;

    event Incremented(address indexed by, uint256 newCount);

    function increment() external {
        count += 1;
        emit Incremented(msg.sender, count);
    }
}`;

const compileScript = `const fs = require('fs');
const hypc = require('@theqrl/hypc');

const input = {
  language: 'Hyperion',
  sources: {
    'Counter.hyp': { content: fs.readFileSync('./Counter.hyp', 'utf8') },
  },
  settings: { outputSelection: { '*': { '*': ['*'] } } },
};

const output = JSON.parse(hypc.compile(JSON.stringify(input)));
const artifact = output.contracts['Counter.hyp']['Counter'];

fs.writeFileSync('Counter.json', JSON.stringify({
  abi: artifact.abi,
  bytecode: artifact.zvm.bytecode.object,
}));`;

const deployScript = `const { Web3 } = require('@theqrl/web3');
const { MLDSA87 } = require('@theqrl/wallet.js');
const { abi, bytecode } = require('./Counter.json');

const web3 = new Web3(new Web3.providers.HttpProvider(
  'https://qrlwallet.com/api/qrl-rpc/testnet',
));

// A 34-word test mnemonic. Never hardcode one; never reuse a real wallet.
const mnemonic = process.env.MNEMONIC;
const hexseed = MLDSA87.newWalletFromMnemonic(mnemonic).getHexExtendedSeed();
const acc = web3.qrl.accounts.seedToAccount(hexseed);
web3.qrl.wallet.add(hexseed);

async function main() {
  const chainId = BigInt(await web3.qrl.getChainId());
  if (chainId !== 1337n) throw new Error('unexpected chain: ' + chainId);

  const deploy = new web3.qrl.Contract(abi)
    .deploy({ data: bytecode, arguments: [] });
  const estimated = await deploy.estimateGas({ from: acc.address });

  const receipt = await web3.qrl.sendTransaction({
    from: acc.address,
    data: deploy.encodeABI(),
    gas: (estimated * 12n) / 10n, // 20% headroom
    gasPrice: await web3.qrl.getGasPrice(),
  });
  console.log('Contract address:', receipt.contractAddress);
}

main().catch((e) => { console.error(e); process.exitCode = 1; });`;

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="deploy-and-verify-contract">
      <p>
        QRL 2.0 runs an EVM compatible execution layer, so deploying a smart contract feels
        familiar if you have shipped one on any Ethereum style chain. The toolchain differs in
        two places: contracts are compiled with <strong>Hyperion</strong> (<code>hypc</code>),
        the QRL toolchain&apos;s contract compiler, and transactions are signed with
        post-quantum ML-DSA-87 keys held by Q-prefixed accounts.
      </p>
      <p>
        This tutorial walks the full loop: write a small Hyperion contract, compile it with{' '}
        <code>hypc</code>, deploy it to the QRL 2.0 testnet with a short{' '}
        <code>@theqrl/web3</code> script, and then verify the source on ZondScan so anyone can
        read it, call it, and audit it. If the chain itself is new to you, start with{' '}
        <a href="/learn/what-is-qrl-2">What is QRL 2.0?</a>.
      </p>
      <p>
        You need Node.js, a funded testnet account, and about twenty minutes. If your goal is a
        standard token without writing contract code, the wallet has a one-click path: see{' '}
        <a href="/learn/launch-a-qrc20">Launch a QRC20 token</a>.
      </p>

      <Callout type="info" title="This is a public testnet">
        <p>
          QRL 2.0 currently runs as a public testnet; mainnet arrives after the migration from
          the legacy chain. Gas is paid in testnet Quanta, which has no monetary value. Fund
          your deployer address for free at the <Link href="/faucet">faucet</Link> (walkthrough in{' '}
          <a href="/learn/get-testnet-quanta">Get testnet Quanta</a>).
        </p>
      </Callout>

      <LearnSection id="hyperion-and-hypc" title="Hyperion and hypc">
        <p>
          Hyperion is a statically typed, contract-oriented language derived from Solidity, and
          <code>hypc</code> is its compiler. Source files use the <code>.hyp</code> extension
          and open with a <code>pragma hyperion</code> line. The compiler is developed in the
          open in the <a href="https://github.com/theQRL">theQRL GitHub organization</a> and is
          published to npm as <code>@theqrl/hypc</code>, so a plain Node.js project can compile
          contracts without any extra toolchain.
        </p>
        <p>
          The compiled bytecode is regular EVM bytecode targeting the chain&apos;s virtual
          machine, which is why the deploy flow below looks like any web3 deploy. In the
          compiler&apos;s standard JSON output the artifacts sit under a <code>zvm</code> key,
          so the bytecode path is <code>contracts[file][name].zvm.bytecode.object</code>.
        </p>
      </LearnSection>

      <LearnSection id="write-a-simple-contract" title="Write a simple contract">
        <p>
          Save the following as <code>Counter.hyp</code>. It stores one number, exposes a
          public getter, and emits an event on every increment, enough surface to see the
          verified Code, Read, and Write tabs in action later.
        </p>
        <CodeBlock code={contractSource} language="hyperion" />
        <p>
          If you know Solidity you can read this without a manual. Existing contracts
          generally port by renaming the file to <code>.hyp</code> and switching the pragma;
          standard ERC20 style sources have been ported this way on this network.
        </p>
      </LearnSection>

      <LearnSection id="compile-with-hypc" title="Compile with hypc">
        <p>
          <code>hypc</code> accepts the standard JSON input format: a <code>language</code>{' '}
          field set to <code>Hyperion</code>, a <code>sources</code> map keyed by filename, and
          a <code>settings</code> block for the optimizer and output selection. This script
          compiles the contract and writes the ABI and bytecode to a JSON artifact:
        </p>
        <CodeBlock code={compileScript} language="javascript" />
        <p>
          Record the exact <code>@theqrl/hypc</code> version and optimizer settings you compile
          with. Verification recompiles your source and byte-matches the result, so those
          settings must match at verification time.
        </p>
      </LearnSection>

      <LearnSection id="deploy-to-the-testnet" title="Deploy to the testnet">
        <p>
          The deploy script signs with an ML-DSA-87 account derived from a 34-word mnemonic and
          talks to the public testnet RPC proxy. The QRL 2.0 testnet uses chain ID{' '}
          <code>1337</code>, and the script aborts on a mismatch before signing anything.
        </p>
        <CodeBlock code={deployScript} language="javascript" />
        <p>
          Run it with <code>MNEMONIC</code> set in your environment. On success it prints the
          new Q-prefixed contract address. Paste that address into the ZondScan search bar to
          see the creation transaction, or open the deployment hash directly;{' '}
          <a href="/learn/read-a-transaction">How to read a transaction</a> explains every
          field, and <Link href="/gas">the gas page</Link> shows current prices. Your contract also
          appears in the <Link href="/contracts">contracts list</Link>.
        </p>
      </LearnSection>

      <LearnSection id="verify-on-zondscan" title="Verify the source on ZondScan">
        <p>
          Right after deployment the explorer only knows your contract&apos;s bytecode.
          Verification publishes the source: you submit it, the backend recompiles it with the
          pinned Hyperion build you select, and byte-matches the output against the on-chain
          runtime code. On success the source, ABI, and compiler settings appear on the
          contract&apos;s page.
        </p>
        <StepList>
          <Step title="Open the verify form">
            <p>
              Go to <Link href="/verify-contract">/verify-contract</Link>. The form loads the list of
              supported compiler builds from the backend and preselects the default.
            </p>
          </Step>
          <Step title="Pick the compiler build">
            <p>
              Choose the exact Hyperion build your contract was compiled with. The byte-match
              only succeeds against that build, so a version mismatch is the most common cause
              of a failed verification.
            </p>
          </Step>
          <Step title="Fill in address, name, and settings">
            <p>
              Enter the Q-prefixed contract address and the contract name, which must match a
              contract declared in the source (<code>Counter</code> here). Set the optimizer
              checkbox and runs to whatever you compiled with, pick a license, and optionally
              set an EVM version.
            </p>
          </Step>
          <Step title="Paste the source">
            <p>
              Paste the full <code>.hyp</code> source into the source field. Multi-file
              projects put the main file there and supply the rest through the Imports field as
              a JSON object mapping import paths to source contents, for example{' '}
              <code>{'{"Context.hyp": "...source..."}'}</code>. Constructor arguments are an
              optional hex field; the byte-match runs against the deployed runtime code either
              way.
            </p>
          </Step>
          <Step title="Submit and watch the job">
            <p>
              Press <strong>Verify &amp; Publish</strong>. Verification runs as an async job:
              the page shows it moving from queued to compiling and polls until it lands on
              verified or failed. Failures come back with the compiler error text so you can
              fix the input and resubmit. If someone already verified the address, the form
              says so and links straight to the contract page.
            </p>
          </Step>
        </StepList>
      </LearnSection>

      <LearnSection id="what-verification-unlocks" title="What verification unlocks">
        <p>
          A verified contract gets a verified badge on its address page, and its Contract
          section grows three tabs:
        </p>
        <ul>
          <li>
            <strong>Code</strong>: the published source with license header, the compiler
            settings used for the match, the full ABI, and the deployed bytecode.
          </li>
          <li>
            <strong>Read</strong>: every <code>view</code> and <code>pure</code> function
            becomes callable directly from the explorer through a read-only call proxy,
            without connecting a wallet.
          </li>
          <li>
            <strong>Write</strong>: state-changing functions can be dispatched by pairing a
            wallet over QRL Connect.
          </li>
        </ul>
        <p>
          Verified contracts also unlock an AI explanation card on the Code tab, which produces
          a plain-language summary of what the contract does. The explainer only works with
          verified source, one more reason to publish it.
        </p>
      </LearnSection>

      <LearnSection id="verify-programmatically" title="Verify programmatically">
        <p>
          Everything the form does is available over the public API at{' '}
          <code>https://zondscan.com/api</code>, which suits CI pipelines that verify every
          deployment automatically:
        </p>
        <ul>
          <li>
            <code>GET /api/contract/compiler-info</code> lists the supported Hyperion builds
            and marks the default.
          </li>
          <li>
            <code>POST /api/contract/verify</code> enqueues a job. The JSON body requires{' '}
            <code>address</code>, <code>sourceCode</code>, and <code>contractName</code>;
            optional fields include <code>compilerVersion</code>, <code>optimizerEnabled</code>
            , <code>optimizerRuns</code>, <code>evmVersion</code>, <code>imports</code>, and{' '}
            <code>license</code>. It returns a <code>jobId</code>. An unknown{' '}
            <code>compilerVersion</code> returns 400 with the supported build list. The
            endpoint is rate-limited per IP and caps the request body at 1 MiB.
          </li>
          <li>
            <code>GET /api/contract/verify/{'{jobId}'}</code> polls the job until it reports
            success or failed.
          </li>
        </ul>
        <p>
          The <Link href="/api-explorer">API explorer</Link> documents these endpoints alongside the
          rest of the public API, with example requests you can run in the browser.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Can I reuse my existing Solidity contracts?',
            a: (
              <p>
                Largely yes. Hyperion is derived from Solidity, so typical contracts port by
                renaming files to <code>.hyp</code> and switching the pragma to{' '}
                <code>pragma hyperion</code>. Test the compile with <code>@theqrl/hypc</code>{' '}
                and fix anything the compiler flags.
              </p>
            ),
          },
          {
            q: 'Verification fails even though my source is correct. Why?',
            a: (
              <p>
                The byte-match requires the exact compiler build and the exact optimizer
                settings used at deploy time. Check the build list on the form or at{' '}
                <code>/api/contract/compiler-info</code>, and make sure the optimizer checkbox
                and runs value match your compile. The failed job includes the compiler output
                to help you diagnose the difference.
              </p>
            ),
          },
          {
            q: 'Do I need to supply constructor arguments to verify?',
            a: (
              <p>
                No. The field is optional and advisory. The match runs against the deployed
                runtime code, which does not include constructor arguments.
              </p>
            ),
          },
          {
            q: 'Does verification cost anything?',
            a: (
              <p>
                Verification is free. Deployment costs gas in testnet Quanta, which you can get
                for free from the <Link href="/faucet">faucet</Link> and which carries no monetary
                value.
              </p>
            ),
          },
          {
            q: 'My contract imports several files. How do I verify it?',
            a: (
              <p>
                Put the main file in the source field and the imported files in the Imports
                field as a JSON object mapping each import path to its full source, matching
                the paths your import statements use. The API accepts the same structure in the{' '}
                <code>imports</code> body field.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
