package db

// import (
// 	"QRLtoMongoDB/configs"

// 	web3 "github.com/chenzhijie/go-web3"
// )

// func ContractCall() {
// 	web3, err := web3.NewWeb3(configs.Url)
// 	if err != nil {
// 		panic(err)
// 	}

// 	    // Get solidity compiled contract output
// 		let output = contractCompiler.GetCompilerOutput()
// 		console.log("Compiler output: ", output);

// 		const inputABI = output.contracts['MyVote.sol']['Vote'].abi;
// 		const deployedContractAddress = config.contract  // Deployed contract address

// 	abiString := `[
// 	{
// 		"constant": true,
// 		"inputs": [],
// 		"name": "totalSupply",
// 		"outputs": [
// 			{
// 				"name": "",
// 				"type": "uint256"
// 			}
// 		],
// 		"payable": false,
// 		"stateMutability": "view",
// 		"type": "function"
// 	}
// ]`
// 	contractAddr := "0xcEF0271647F8887358e00527aB8D9205a87F5fd3" // contract address
// 	contract, err := web3.Eth.NewContract(abiString, contractAddr)
// 	if err != nil {
// 		panic(err)
// 	}
// }
