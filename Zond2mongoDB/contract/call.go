package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	solc "github.com/imxyb/solc-go"
	web3 "github.com/successor1/go-web3"
)

type Config struct {
	Hexseed  string `json:"hexseed"`
	Provider string `json:"provider"`
	Contract string `json:"contract"`
}

func main() {
	// Load configuration values from a JSON file
	var config Config
	data, err := os.ReadFile("./config.json")
	if err != nil {
		log.Fatalf("Error reading config.json: %v", err)
	}
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Error unmarshalling config: %v", err)
	}

	// Check the hexseed value
	if config.Hexseed == "hexseed_here" {
		fmt.Println("You need to enter a dilithium hexseed for this to work.")
		os.Exit(1)
	}

	// Get the specified Solidity compiler version
	compiler, err := solc.GetCompiler("0.8.21")
	if err != nil {
		log.Fatalf("Error getting compiler: %v", err)
	}

	// Compile the contract
	myVoteContent, err := os.ReadFile("./contracts/MyVote.sol")
	if err != nil {
		log.Fatalf("Error reading MyVote.sol: %v", err)
	}

	input := &solc.Input{
		Language: "Solidity",
		Sources: map[string]solc.SourceIn{
			"MyVote.sol": {Content: string(myVoteContent)},
		},
		Settings: solc.Settings{
			OutputSelection: map[string]map[string][]string{
				"*": {
					"*": []string{"*"},
				},
			},
		},
	}

	output, err := compiler.Compile(input)
	if err != nil {
		log.Fatalf("Error compiling: %v", err)
	}

	// Interact with the Ethereum contract
	w3, err := web3.NewWeb3(config.Provider)
	if err != nil {
		log.Fatalf("Error initializing web3: %v", err)
	}

	metadataString := output.Contracts["MyVote.sol"]["Vote"].Metadata
	metadataMap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(metadataString), &metadataMap); err != nil {
		log.Fatalf("Error unmarshalling metadata: %v", err)
	}

	abiInterface, ok := metadataMap["output"].(map[string]interface{})["abi"]
	if !ok {
		log.Fatalf("Unable to extract ABI from metadata")
	}

	abiBytes, err := json.Marshal(abiInterface)
	if err != nil {
		log.Fatalf("Error marshalling ABI: %v", err)
	}

	abiString := string(abiBytes)

	contractAddr := config.Contract
	contract, err := w3.Eth.NewContract(abiString, contractAddr)
	if err != nil {
		log.Fatalf("Error initializing contract: %v", err)
	}

	methods := contract.Methods("getVotes")

	fmt.Println(methods)

	// voteCountA, err := contract.Call("getVotes", "DARK MODE")
	// if err != nil {
	// 	log.Fatalf("Error fetching DARK MODE votes: %v", err)
	// }

	// fmt.Printf("Vote count for DARK MODE: %v\n", voteCountA)

	// voteCountB, err := contract.Call("getVotes", "LIGHT MODE")
	// if err != nil {
	// 	log.Fatalf("Error fetching LIGHT MODE votes: %v", err)
	// }

	// fmt.Printf("Vote count for LIGHT MODE: %v\n", voteCountB)
}
