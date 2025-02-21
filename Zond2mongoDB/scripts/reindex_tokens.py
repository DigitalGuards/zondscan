#!/usr/bin/env python3

import os
import time
from datetime import datetime
import json
import requests
from pymongo import MongoClient
from dotenv import load_dotenv
from web3 import Web3
import logging

# Load environment variables
load_dotenv()

# Set up logging
log_dir = os.path.join(os.path.dirname(__file__), '../logs')
os.makedirs(log_dir, exist_ok=True)
log_file = os.path.join(log_dir, 'reindex_tokens.log')

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.FileHandler(log_file),
        logging.StreamHandler()  # Also log to console
    ]
)

logger = logging.getLogger(__name__)

# MongoDB connection
MONGO_URI = os.getenv('MONGOURI', 'mongodb://localhost:27017')
NODE_URL = os.getenv('NODE_URL', 'http://REDACTED:4545')

# Constants
TRANSFER_EVENT_SIGNATURE = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
BATCH_SIZE = 50  # Reduced from 100 to 50 for faster processing

def make_rpc_call(method, params, max_retries=3, retry_delay=1):
    """Make an RPC call to the Zond node with retries."""
    for attempt in range(max_retries):
        try:
            logger.info(f"Making RPC call: {method} (attempt {attempt + 1}/{max_retries})")
            payload = {
                "jsonrpc": "2.0",
                "method": method,
                "params": params,
                "id": 1
            }
            response = requests.post(NODE_URL, json=payload)
            result = response.json().get('result')
            logger.info(f"RPC response for {method}: {result}")
            return result
        except Exception as e:
            logger.error(f"RPC call failed (attempt {attempt + 1}): {e}")
            if attempt < max_retries - 1:
                logger.info(f"Retrying in {retry_delay} seconds...")
                time.sleep(retry_delay)
                retry_delay *= 2  # Exponential backoff
            else:
                raise

def get_token_balance(contract_address, holder_address):
    """Get token balance for a specific address."""
    # balanceOf(address) signature
    method_sig = "0x70a08231000000000000000000000000" + holder_address[2:].lower().zfill(40)
    result = make_rpc_call("zond_call", [{
        "to": contract_address,
        "data": method_sig
    }, "latest"])
    
    if result:
        return Web3.to_int(hexstr=result)
    return 0

def get_logs(contract_address, from_block, to_block):
    """Get Transfer event logs for a contract."""
    return make_rpc_call("zond_getLogs", [{
        "address": contract_address,
        "topics": [TRANSFER_EVENT_SIGNATURE],
        "fromBlock": hex(from_block),
        "toBlock": hex(to_block)
    }])

def process_transfer_logs(logs, contract_address, token_balances_collection):
    """Process Transfer event logs and update balances."""
    if not logs:
        return

    # Track unique addresses that need balance updates
    addresses_to_update = set()
    
    for log in logs:
        try:
            # Extract from and to addresses from topics
            from_addr = "0x" + log["topics"][1][-40:]
            to_addr = "0x" + log["topics"][2][-40:]
            
            addresses_to_update.add(from_addr.lower())
            addresses_to_update.add(to_addr.lower())
        except (KeyError, IndexError) as e:
            logger.error(f"Error processing log: {e}")
            logger.error(f"Log data: {log}")
            continue
    
    # Update balances for all affected addresses
    for address in addresses_to_update:
        logger.info(f"Getting balance for {address}...")
        current_balance = get_token_balance(contract_address, address)
        if current_balance is not None:
            try:
                # Update in MongoDB
                token_balances_collection.update_one(
                    {
                        "contractAddress": contract_address.lower(),
                        "holderAddress": address.lower()
                    },
                    {
                        "$set": {
                            "balance": str(current_balance),
                            "updatedAt": datetime.utcnow().isoformat()
                        }
                    },
                    upsert=True
                )
                logger.info(f"Updated balance for {address}: {current_balance}")
            except Exception as e:
                logger.error(f"Error updating balance in MongoDB: {e}")
        else:
            logger.info(f"Failed to get balance for {address}")

def get_last_processed_block(token_balances_collection, contract_address):
    """Get the last processed block for a contract."""
    last_balance = token_balances_collection.find_one(
        {"contractAddress": contract_address.lower()},
        sort=[("blockNumber", -1)]
    )
    if last_balance and "blockNumber" in last_balance:
        try:
            return int(last_balance["blockNumber"], 16)
        except ValueError:
            return 0
    return 0

def main():
    logger.info("Starting token reindexing...")
    
    # Connect to MongoDB
    logger.info(f"Connecting to MongoDB at {MONGO_URI}")
    client = MongoClient(MONGO_URI)
    db = client['qrldata-b2h']  # Changed from qrldata to qrldata-b2h to match Go code
    
    # Get collections
    contracts_collection = db.contractCode  # Changed from contracts to contractCode
    token_balances_collection = db.tokenBalances  # Changed from tokenbalances to tokenBalances
    
    # Check all contracts
    logger.info("\nChecking all contracts in database:")
    all_contracts = contracts_collection.find({})
    all_count = contracts_collection.count_documents({})
    logger.info(f"Total contracts found: {all_count}")
    
    token_contracts = list(contracts_collection.find({"isToken": True}))
    token_count = len(token_contracts)
    logger.info(f"\nFound {token_count} token contracts")
    
    # Get latest block number
    logger.info("\nGetting latest block number...")
    latest_block = int(make_rpc_call("zond_blockNumber", []), 16)
    logger.info(f"Latest block: {latest_block}")
    
    for i, contract in enumerate(token_contracts, 1):
        contract_address = contract['address']
        logger.info(f"\nProcessing token {i}/{token_count}: {contract.get('name', 'Unknown')} ({contract_address})")
        
        # Get creation block number and last processed block
        creation_block = int(contract.get('creationBlockNumber', '0x0'), 16)
        last_processed = get_last_processed_block(token_balances_collection, contract_address)
        start_block = max(creation_block, last_processed + 1)
        
        logger.info(f"Creation block: {creation_block}")
        logger.info(f"Last processed block: {last_processed}")
        logger.info(f"Starting from block: {start_block}")
        
        total_transfers = 0
        current_block = start_block
        
        while current_block < latest_block:
            end_block = min(current_block + BATCH_SIZE, latest_block)
            
            logger.info(f"Processing blocks {current_block} to {end_block} ({((end_block - start_block) / (latest_block - start_block)) * 100:.1f}% complete)...")
            logs = get_logs(contract_address, current_block, end_block)
            
            if logs:
                transfer_count = len(logs)
                total_transfers += transfer_count
                logger.info(f"Found {transfer_count} transfer events (total: {total_transfers})")
                process_transfer_logs(logs, contract_address, token_balances_collection)
            
            current_block = end_block + 1
            time.sleep(0.1)  # Rate limiting
        
        logger.info(f"Completed processing for {contract_address} - Total transfers: {total_transfers}")

if __name__ == "__main__":
    main()
