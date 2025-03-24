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
NODE_URL = os.getenv('NODE_URL', 'https://qrlwallet.com/api/zond-rpc/testnet')

# Constants
TRANSFER_EVENT_SIGNATURE = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
BATCH_SIZE = 50

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

def process_transfer_logs(logs, contract_address, token_balances_collection, token_transfers_collection, contract):
    """Process Transfer event logs and update balances."""
    for log in logs:
        # Extract transfer details
        topics = log.get('topics', [])
        if len(topics) != 3:  # Transfer event has 3 topics
            continue
        
        try:
            from_addr = 'Z' + topics[1][-40:]  # Remove padding
            to_addr = 'Z' + topics[2][-40:]  # Remove padding
            amount = int(log.get('data', '0x0'), 16)
            block_number = log.get('blockNumber')
            tx_hash = log.get('transactionHash')
            
            # Get block timestamp
            block = make_rpc_call("zond_getBlockByNumber", [block_number, False])
            if not block:
                logger.error(f"Could not get block {block_number}")
                continue
            block_timestamp = block.get('timestamp')
            if not block_timestamp:
                logger.error(f"No timestamp in block {block_number}")
                continue
            
            # Store token transfer
            transfer = {
                'contractAddress': contract_address,
                'from': from_addr,
                'to': to_addr,
                'amount': str(amount),  # Convert to string to match Go model
                'blockNumber': block_number,
                'txHash': tx_hash,
                'timestamp': block_timestamp,
                'tokenSymbol': contract.get('symbol', ''),
                'tokenDecimals': contract.get('decimals', 0),
                'tokenName': contract.get('name', ''),
                'transferType': 'event'
            }
            
            try:
                token_transfers_collection.insert_one(transfer)
            except Exception as e:
                if 'duplicate key error' not in str(e):  # Ignore duplicates
                    logger.error(f"Failed to store transfer {tx_hash}: {e}")
            
            # Update balances as strings to avoid integer overflow
            if from_addr != '0x0000000000000000000000000000000000000000' and from_addr != 'Z0000000000000000000000000000000000000000':
                # Get current balance
                current_from_balance_doc = token_balances_collection.find_one(
                    {'contractAddress': contract_address, 'holderAddress': from_addr}
                )
                
                current_from_balance = 0
                if current_from_balance_doc and 'balance' in current_from_balance_doc:
                    # Convert existing balance to integer if it's a string
                    if isinstance(current_from_balance_doc['balance'], str):
                        try:
                            current_from_balance = int(current_from_balance_doc['balance'])
                        except ValueError:
                            current_from_balance = 0
                    else:
                        current_from_balance = current_from_balance_doc['balance']
                        
                # Calculate new balance and store as string
                new_from_balance = max(0, current_from_balance - amount)
                token_balances_collection.update_one(
                    {'contractAddress': contract_address, 'holderAddress': from_addr},
                    {'$set': {'balance': str(new_from_balance)}},
                    upsert=True
                )
            
            if to_addr != '0x0000000000000000000000000000000000000000' and to_addr != 'Z0000000000000000000000000000000000000000':
                # Get current balance
                current_to_balance_doc = token_balances_collection.find_one(
                    {'contractAddress': contract_address, 'holderAddress': to_addr}
                )
                
                current_to_balance = 0
                if current_to_balance_doc and 'balance' in current_to_balance_doc:
                    # Convert existing balance to integer if it's a string
                    if isinstance(current_to_balance_doc['balance'], str):
                        try:
                            current_to_balance = int(current_to_balance_doc['balance'])
                        except ValueError:
                            current_to_balance = 0
                    else:
                        current_to_balance = current_to_balance_doc['balance']
                        
                # Calculate new balance and store as string
                new_to_balance = current_to_balance + amount
                token_balances_collection.update_one(
                    {'contractAddress': contract_address, 'holderAddress': to_addr},
                    {'$set': {'balance': str(new_to_balance)}},
                    upsert=True
                )
            
            # Update last processed block
            token_balances_collection.update_one(
                {'contractAddress': contract_address, 'holderAddress': 'lastProcessedBlock'},
                {'$set': {'balance': int(block_number, 16)}},
                upsert=True
            )
            
        except Exception as e:
            logger.error(f"Error processing transfer in block {block_number}: {str(e)}", exc_info=True)
            continue

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

# Helper function to validate address format
def is_valid_address(address):
    # Check for Z-prefix format
    if address.startswith('Z'):
        # Check if the rest is valid hex
        try:
            int(address[1:], 16)
            return len(address) == 41  # Z + 40 hex chars
        except ValueError:
            return False
    
    # Check for 0x format
    if address.startswith('0x'):
        try:
            int(address[2:], 16)
            return len(address) == 42  # 0x + 40 hex chars
        except ValueError:
            return False
    
    return False

# Helper function to normalize address format
def normalize_address(address):
    # If it's already a valid address, return it
    if is_valid_address(address):
        return address
    
    # Try to add 0x prefix if it's a valid hex string
    try:
        int(address, 16)
        if len(address) == 40:
            return '0x' + address
    except ValueError:
        pass
    
    return address

def update_missing_creation_blocks(contracts_collection):
    """Update contracts with missing creation block numbers."""
    logger.info("Checking for contracts with missing creation block numbers...")
    
    # Find contracts with missing or empty creation block numbers
    missing_blocks = list(contracts_collection.find({
        "$or": [
            {"creationBlockNumber": ""},
            {"creationBlockNumber": {"$exists": False}}
        ]
    }))
    
    if not missing_blocks:
        logger.info("No contracts with missing creation block numbers found.")
        return
    
    logger.info(f"Found {len(missing_blocks)} contracts with missing creation block numbers.")
    
    updated_count = 0
    for contract in missing_blocks:
        # If we have a creation transaction hash, get its block number
        if contract.get('creationTransaction'):
            tx_hash = contract['creationTransaction']
            try:
                # Get transaction receipt to find the block number
                receipt = make_rpc_call("zond_getTransactionReceipt", [tx_hash])
                if receipt and receipt.get('blockNumber'):
                    block_number = receipt['blockNumber']
                    
                    # Update the contract
                    contracts_collection.update_one(
                        {"address": contract['address']},
                        {"$set": {"creationBlockNumber": block_number}}
                    )
                    
                    logger.info(f"Updated creation block number for {contract['address']} to {block_number}")
                    updated_count += 1
                else:
                    logger.warning(f"Could not find block number for transaction {tx_hash}")
            except Exception as e:
                logger.error(f"Error updating creation block for {contract['address']}: {str(e)}")
    
    logger.info(f"Updated creation block numbers for {updated_count} contracts.")

def main():
    logger.info("Starting token reindexing...")
    
    # Connect to MongoDB
    logger.info(f"Connecting to MongoDB at {MONGO_URI}")
    client = MongoClient(MONGO_URI)
    db = client['qrldata-z']  # Changed from qrldata to qrldata-z to match Go code
    
    # Get collections
    contracts_collection = db.contractCode  # Changed from contracts to contractCode
    token_balances_collection = db.tokenBalances  # Changed from tokenbalances to tokenBalances
    token_transfers_collection = db.tokenTransfers  # New collection for token transfers
    
    # Create indexes for token transfers collection
    token_transfers_collection.create_index([("contractAddress", 1), ("blockNumber", 1)])
    token_transfers_collection.create_index([("from", 1), ("blockNumber", 1)])
    token_transfers_collection.create_index([("to", 1), ("blockNumber", 1)])
    token_transfers_collection.create_index([("txHash", 1)], unique=True)
    
    # Check all contracts
    logger.info("\nChecking all contracts in database:")
    all_contracts = contracts_collection.find({})
    all_count = contracts_collection.count_documents({})
    logger.info(f"Total contracts found: {all_count}")
    
    # Update missing creation block numbers
    update_missing_creation_blocks(contracts_collection)
    
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
        creation_block_hex = contract.get('creationBlockNumber', '0x0')
        # Handle empty or invalid creation block numbers
        if not creation_block_hex or creation_block_hex == '':
            creation_block_hex = '0x0'
            # If we have a creation transaction, we could try to find the block number
            if contract.get('creationTransaction'):
                logger.info(f"Empty creation block number but has creation transaction. Consider reprocessing contracts.")
        
        try:
            creation_block = int(creation_block_hex, 16)
        except ValueError:
            logger.warning(f"Invalid creationBlockNumber '{creation_block_hex}' for contract {contract_address}, using 0 instead")
            creation_block = 0
            
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
                process_transfer_logs(logs, contract_address, token_balances_collection, token_transfers_collection, contract)
            
            current_block = end_block + 1
            time.sleep(0.1)  # Rate limiting
        
        logger.info(f"Completed processing for {contract_address} - Total transfers: {total_transfers}")

if __name__ == "__main__":
    main()
