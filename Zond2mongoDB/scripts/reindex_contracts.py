#!/usr/bin/env python3

import os
import time
from datetime import datetime
import json
import requests
from pymongo import MongoClient
from dotenv import load_dotenv
import logging

# Load environment variables
load_dotenv()

# Set up logging
log_dir = os.path.join(os.path.dirname(__file__), '../logs')
os.makedirs(log_dir, exist_ok=True)
log_file = os.path.join(log_dir, 'reindex_contracts.log')

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

def make_rpc_call(method, params):
    """Make an RPC call to the Zond node."""
    headers = {'content-type': 'application/json'}
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": 1
    }
    
    try:
        response = requests.post(NODE_URL, json=payload, headers=headers)
        return response.json().get('result')
    except Exception as e:
        logger.error(f"RPC call failed: {e}")
        return None

def get_contract_code(address):
    """Get contract bytecode from the node."""
    return make_rpc_call("zond_getCode", [address, "latest"])

def get_transaction_receipt(tx_hash):
    """Get transaction receipt from the node."""
    return make_rpc_call("zond_getTransactionReceipt", [tx_hash])

def call_contract_method(contract_address, method_signature):
    """Call a contract method using eth_call."""
    data = {
        "to": contract_address,
        "data": method_signature
    }
    return make_rpc_call("zond_call", [data, "latest"])

def get_token_info(contract_address):
    """Get ERC20 token information from a contract."""
    # Method signatures for ERC20 interface
    NAME_SIG = "0x06fdde03"      # name()
    SYMBOL_SIG = "0x95d89b41"    # symbol()
    DECIMALS_SIG = "0x313ce567"  # decimals()
    
    name = symbol = ""
    decimals = 0
    is_token = False
    
    # Try to get token name
    name_result = call_contract_method(contract_address, NAME_SIG)
    if name_result and len(name_result) >= 66:
        try:
            # Remove function selector and length prefix, and handle dynamic strings
            name_hex = name_result[66:].rstrip("0")
            if name_hex:
                # Handle both fixed and dynamic strings
                try:
                    # Try to decode as dynamic string first
                    offset = int(name_result[2:66], 16) * 2  # Convert offset to hex string position
                    length = int(name_result[offset+2:offset+66], 16) * 2  # Get string length
                    name_hex = name_result[offset+66:offset+66+length].rstrip("0")
                except:
                    # If dynamic string parsing fails, try fixed string
                    name_hex = name_result[66:].rstrip("0")
                
                if name_hex:
                    name = bytes.fromhex(name_hex).decode('utf-8').strip()
                    is_token = True
                    logger.info(f"Decoded token name for {contract_address}: '{name}'")
        except Exception as e:
            logger.error(f"Error decoding name for {contract_address}: {e}")
    
    # Try to get token symbol
    symbol_result = call_contract_method(contract_address, SYMBOL_SIG)
    if symbol_result and len(symbol_result) >= 66:
        try:
            # Handle both fixed and dynamic strings
            try:
                # Try to decode as dynamic string first
                offset = int(symbol_result[2:66], 16) * 2
                length = int(symbol_result[offset+2:offset+66], 16) * 2
                symbol_hex = symbol_result[offset+66:offset+66+length].rstrip("0")
            except:
                # If dynamic string parsing fails, try fixed string
                symbol_hex = symbol_result[66:].rstrip("0")
            
            if symbol_hex:
                symbol = bytes.fromhex(symbol_hex).decode('utf-8').strip()
                is_token = True
                logger.info(f"Decoded token symbol for {contract_address}: '{symbol}'")
        except Exception as e:
            logger.error(f"Error decoding symbol for {contract_address}: {e}")
    
    # Try to get decimals
    decimals_result = call_contract_method(contract_address, DECIMALS_SIG)
    if decimals_result and len(decimals_result) >= 2:
        try:
            decimals = int(decimals_result[2:], 16)
            is_token = True
        except Exception as e:
            logger.error(f"Error decoding decimals for {contract_address}: {e}")
    
    return name, symbol, decimals, is_token

def process_contract_creation(transfer_doc, contracts_collection):
    """Process a contract creation transaction and store contract information."""
    # Get transaction hash in hex format
    tx_hash = "0x" + transfer_doc['txHash'].hex() if isinstance(transfer_doc['txHash'], bytes) else transfer_doc['txHash']
    
    # Get transaction receipt to find contract address
    receipt = get_transaction_receipt(tx_hash)
    if not receipt or not receipt.get('contractAddress'):
        logger.error(f"No contract address found for transaction {tx_hash}")
        return False
        
    contract_address = receipt['contractAddress'].lower()  # Store as lowercase hex
    creator_address = ("Z" + transfer_doc['from'].hex() if isinstance(transfer_doc['from'], bytes) else transfer_doc['from']).lower()
    
    # Get contract code
    contract_code = get_contract_code(contract_address)
    if not contract_code or contract_code == "0x":
        logger.error(f"No code found for contract {contract_address}")
        return False
    
    # Get token information
    name, symbol, decimals, is_token = get_token_info(contract_address)
    
    # Create contract document
    contract_doc = {
        "address": contract_address,  # Store as hex string
        "creatorAddress": creator_address,
        "code": contract_code,
        "creationTransaction": tx_hash,
        "status": receipt.get('status', '0x1'),  # Default to success if status not present
        "isToken": is_token,
        "name": name if is_token else "",
        "symbol": symbol if is_token else "",
        "decimals": decimals if is_token else 0,
        "creationBlockNumber": transfer_doc.get('blockNumber', '0x0'),
        "updatedAt": datetime.utcnow().isoformat()
    }
    
    # Log contract details before storing
    if is_token:
        logger.info(f"Storing token contract - Address: {contract_address}, Name: '{name}', Symbol: '{symbol}', Decimals: {decimals}")
    
    # Insert or update contract
    try:
        result = contracts_collection.update_one(
            {"address": contract_doc["address"]},
            {"$set": contract_doc},
            upsert=True
        )
        logger.info(f"MongoDB update result - Matched: {result.matched_count}, Modified: {result.modified_count}, Upserted: {result.upserted_id is not None}")
    except Exception as e:
        logger.error(f"Error updating contract in MongoDB: {e}")
        return False
    
    return True

def main():
    # Connect to MongoDB
    logger.info(f"Connecting to MongoDB at {MONGO_URI}")
    client = MongoClient(MONGO_URI)
    db = client['qrldata-z']
    
    # Get collections
    contracts_collection = db.contractCode
    transfer_collection = db.transfer
    
    # Print available databases and collections
    logger.info("\nAvailable databases: %s", client.list_database_names())
    logger.info("Available collections: %s", db.list_collection_names())
    
    # Sample some documents from transfer collection
    logger.info("\nSample documents from transfer collection:\n")
    sample_transfers = list(transfer_collection.find().limit(1))
    for doc in sample_transfers:
        logger.info("Document:")
        for key, value in doc.items():
            logger.info(f"{key}: {value}")
    
    # Process contract creations
    logger.info("\nProcessing contract creations...")
    transfers = transfer_collection.find({"contractAddress": {"$exists": True}})
    total_transfers = transfer_collection.count_documents({"contractAddress": {"$exists": True}})
    contracts_created = 0
    
    logger.info(f"\nFound {total_transfers} potential contract creation transactions")
    
    for i, transfer in enumerate(transfers, 1):
        try:
            # Get the contract address directly from the document
            contract_address = transfer.get('contractAddress')
            if not contract_address:
                logger.error(f"No contract address found in transfer document")
                continue
                
            creator_address = transfer.get('from')
            if not creator_address:
                logger.error(f"No creator address found in transfer document")
                continue
            
            # Convert bytes to hex strings if needed
            if isinstance(contract_address, bytes):
                contract_address = "Z" + contract_address.hex()
            if isinstance(creator_address, bytes):
                creator_address = "Z" + creator_address.hex()
                
            contract_address = contract_address.lower()
            creator_address = creator_address.lower()
            
            logger.info(f"Processing contract creation - Contract: {contract_address}, Creator: {creator_address}")
            
            # Get contract code
            contract_code = get_contract_code(contract_address)
            if not contract_code or contract_code == "0x":
                logger.error(f"No code found for contract {contract_address}")
                continue
            
            # Get token information
            name, symbol, decimals, is_token = get_token_info(contract_address)
            if is_token:
                logger.info(f"Found token contract: Name='{name}', Symbol='{symbol}', Decimals={decimals}")
            
            # Create contract document
            contract_doc = {
                "address": contract_address,
                "creatorAddress": creator_address,
                "code": contract_code,
                "creationTransaction": ("0x" + transfer['txHash'].hex() if isinstance(transfer['txHash'], bytes) else transfer['txHash']),
                "status": transfer.get('status', '0x1'),
                "isToken": is_token,
                "name": name if is_token else "",
                "symbol": symbol if is_token else "",
                "decimals": decimals if is_token else 0,
                "creationBlockNumber": transfer.get('blockNumber', '0x0'),
                "updatedAt": datetime.utcnow().isoformat()
            }
            
            # Insert or update contract
            try:
                result = contracts_collection.update_one(
                    {"address": contract_doc["address"]},
                    {"$set": contract_doc},
                    upsert=True
                )
                logger.info(f"Contract {contract_address} updated - Matched: {result.matched_count}, Modified: {result.modified_count}, Upserted: {result.upserted_id is not None}")
                contracts_created += 1
            except Exception as e:
                logger.error(f"Error updating contract in MongoDB: {e}")
                
        except Exception as e:
            logger.error(f"Error processing transfer {i}/{total_transfers}: {str(e)}", exc_info=True)
    
    logger.info("\nReindexing complete!")
    logger.info(f"Total transactions processed: {total_transfers}")
    logger.info(f"Total contracts created: {contracts_created}")

if __name__ == "__main__":
    main()