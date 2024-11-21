# QRLtoMongoDB-PoS

Proof of stake blockchain synchronizer that efficiently transfers QRL blockchain data to MongoDB.

## Overview

This is the Golang implementation of the QRL blockchain synchronizer to MongoDB. It serves as a critical component in the QRL Explorer ecosystem by continuously syncing blockchain data from a Zond node to MongoDB for efficient querying and data access.

## Project Structure

```
├── bitfield/           # Bitfield operations and big number handling
│   ├── big.go
│   └── bitfield.go
│
├── configs/            # Configuration management
│   ├── const.go       # Constants definitions
│   ├── env.go         # Environment variables handling
│   └── setup.go       # Setup and initialization
│
├── contract/          # Smart contract related functionality
│   ├── call.go        # Contract call implementations
│   ├── config.json    # Contract configuration
│   └── contracts/     # Smart contract definitions
│       ├── ERC20.sol
│       ├── IERC20.sol
│       ├── MyToken.sol
│       └── MyVote.sol
│
├── db/               # Database operations and models
│   ├── bitfield.go
│   ├── blocks.go     # Block data handling
│   ├── blocksize.go  # Block size tracking
│   ├── circulating.go
│   ├── coinbase.go   # Coinbase transaction handling
│   ├── coingecko.go  # CoinGecko integration
│   ├── contracts.go  # Smart contract handling
│   ├── conversion.go # Data conversion utilities
│   ├── db.go        # Core database operations
│   ├── transactions.go
│   ├── validators.go # Validator data management
│   ├── volume.go    # Volume tracking
│   └── zond.go      # Zond node interaction
│
├── fetch/           # External data fetching
│   └── coingecko.go # CoinGecko API integration
│
├── logger/          # Logging functionality
│   └── logger.go
│
├── mock_rpc/        # Mock RPC for testing
│   └── myhttpclient_mock.go
│
├── models/          # Data models and structures
│   ├── bitfield.go
│   ├── block.go
│   ├── coingecko.go
│   ├── contract.go
│   ├── db.go
│   ├── trace.go
│   ├── transactions.go
│   ├── transfer.go
│   ├── validators.go
│   ├── wallet.go
│   └── zond.go
│
├── rpc/            # RPC client implementation
│   ├── calls.go    # RPC call definitions
│   └── client.go   # RPC client
│
└── synchroniser/   # Blockchain synchronization
    └── sync.go     # Core sync logic
```

## Setup

1. Create an `.env` file in the root directory with the following fields:

```env
MONGOURI=mongodb://localhost:27017
NODE_URL=http://localhost:8545
```

2. Build the application:
```bash
# On Unix-like systems
go build -o synchroniser main.go

# On Windows
go build -o synchroniser.exe main.go
```

3. Run the synchronizer:
```bash
# On Unix-like systems
./synchroniser

# On Windows
./synchroniser.exe
```

For production deployment, it's recommended to use a process manager like PM2:
```bash
# On Unix-like systems
pm2 start ./synchroniser --name "synchroniser"

# On Windows
pm2 start ./synchroniser.exe --name "synchroniser"
```

## Key Components

- **Synchroniser**: Core component that manages the blockchain synchronization process
- **RPC Client**: Handles communication with the Zond node
- **Database Layer**: Manages MongoDB operations and data persistence
- **Models**: Defines data structures for blockchain entities
- **Contract Handling**: Manages smart contract interactions and events
- **Logging**: Provides detailed logging for monitoring and debugging

## License

This project is licensed under the MIT License - see the LICENSE file for details.
