# Validator System

## Overview
The validator system tracks active validators in the Zond network. It consists of three main components:
1. **Zond2mongoDB**: Fetches and stores validator data from the blockchain
2. **Backend API**: Serves validator data to the frontend
3. **Frontend**: Displays validator information to users

## Epoch Calculation
- Each epoch consists of 128 slots
- Each slot takes 60 seconds (1 minute)
- So each epoch is 128 * 60 = 7,680 seconds (or 2.13 hours)

## Data Flow

### 1. Zond2mongoDB Service
- Located in `db/validators.go`
- Runs every 30 minutes via a periodic task in `synchroniser/sync.go`
- Calculates current epoch: `currentEpoch := blockNumber / 128`
- Updates validator data when:
  - First run
  - Epoch changes
- Stores validator data in MongoDB with structure:
  ```json
  {
    "_id": "validators",
    "jsonrpc": 2,
    "resultvalidator": {
      "epoch": <current_epoch>,
      "validatorsbyslotnumber": [
        {
          "slotnumber": <slot_number>,
          "leader": <leader_address>,
          "attestors": [<attestor_addresses>]
        }
      ]
    }
  }
  ```

### 2. Backend API
- Located in `backendAPI/routes/routes.go`
- Retrieves validator data from MongoDB
- Calculates current epoch using same formula: `blockNumber / 128`
- Returns validator data in format:
  ```json
  {
    "validators": [
      {
        "address": "0x...",
        "uptime": 100.0,
        "age": <current_epoch>,
        "stakedAmount": "40000000000000000000000",
        "isActive": true
      }
    ],
    "totalStaked": <total_staked_amount>
  }
  ```

### 3. Frontend
- Located in `ExplorerFrontend/app/validators/validators-client.tsx`
- Displays validator information in a table/mobile view
- Converts epoch age to days using helper function:
  ```javascript
  epochsToDays(epochs) {
    // Each epoch is 128 slots
    // Each slot takes 60 seconds
    // So each epoch is 128 * 60 seconds
    return (epochs * 128 * 60) / (24 * 60 * 60);
  }
  ```

## Staking
- Each validator stakes 40,000 QUANTA (shown in Wei: 40000000000000000000000)
- Total staked amount is calculated by multiplying individual stake by number of validators

## Future Improvements
1. Calculate actual validator uptime from historical data (currently hardcoded to 100%)
2. Track validator performance metrics
3. Implement slashing detection
4. Add validator rewards tracking



TODO: implement frontend decoding of validator public key to address using wallet.js 