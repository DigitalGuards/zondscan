// The tutorial's deploy script, shown on the page as written. A test runs it
// against a fake node with the installed @theqrl/web3.
export const deployScript = `const { Web3 } = require('@theqrl/web3');
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
  if (chainId !== 3151909n) throw new Error('unexpected chain: ' + chainId);

  const deploy = new web3.qrl.Contract(abi)
    .deploy({ data: bytecode, arguments: [] });
  const estimated = await deploy.estimateGas({ from: acc.address });

  // Type 2 (EIP-1559) transaction: web3 fills maxFeePerGas and
  // maxPriorityFeePerGas from the latest block.
  const receipt = await web3.qrl.sendTransaction({
    type: '0x2',
    from: acc.address,
    data: deploy.encodeABI(),
    gas: (estimated * 12n) / 10n, // 20% headroom
  });
  console.log('Contract address:', receipt.contractAddress);
}

main().catch((e) => { console.error(e); process.exitCode = 1; });`;
