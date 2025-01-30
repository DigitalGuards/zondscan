#!/usr/bin/env python3

import os
import time
from datetime import datetime
import json
import requests
from pymongo import MongoClient
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# MongoDB connection
MONGO_URI = os.getenv('MONGOURI', 'mongodb://localhost:27017')
NODE_URL = os.getenv('NODE_URL', 'http://REDACTED:4545')

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
        print(f"RPC call failed: {e}")
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
            # Remove function selector and length prefix
            name_hex = name_result[66:].rstrip("0")
            if name_hex:
                name = bytes.fromhex(name_hex).decode('utf-8')
                is_token = True
        except Exception as e:
            print(f"Error decoding name for {contract_address}: {e}")
    
    # Try to get token symbol
    symbol_result = call_contract_method(contract_address, SYMBOL_SIG)
    if symbol_result and len(symbol_result) >= 66:
        try:
            symbol_hex = symbol_result[66:].rstrip("0")
            if symbol_hex:
                symbol = bytes.fromhex(symbol_hex).decode('utf-8')
                is_token = True
        except Exception as e:
            print(f"Error decoding symbol for {contract_address}: {e}")
    
    # Try to get decimals
    decimals_result = call_contract_method(contract_address, DECIMALS_SIG)
    if decimals_result and len(decimals_result) >= 2:
        try:
            decimals = int(decimals_result[2:], 16)
            is_token = True
        except Exception as e:
            print(f"Error decoding decimals for {contract_address}: {e}")
    
    return name, symbol, decimals, is_token

def process_contract_creation(transfer_doc, contracts_collection):
    """Process a contract creation transaction and store contract information."""
    # Get transaction hash in hex format
    tx_hash = "0x" + transfer_doc['txHash'].hex() if isinstance(transfer_doc['txHash'], bytes) else transfer_doc['txHash']
    
    # Get transaction receipt to find contract address
    receipt = get_transaction_receipt(tx_hash)
    if not receipt or not receipt.get('contractAddress'):
        print(f"No contract address found for transaction {tx_hash}")
        return False
        
    contract_address = receipt['contractAddress']
    creator_address = "0x" + transfer_doc['from'].hex() if isinstance(transfer_doc['from'], bytes) else transfer_doc['from']
    
    # Get contract code
    contract_code = get_contract_code(contract_address)
    if not contract_code or contract_code == "0x":
        print(f"No code found for contract {contract_address}")
        return False
    
    # Get token information
    name, symbol, decimals, is_token = get_token_info(contract_address)
    
    # Create contract document
    contract_doc = {
        "contractAddress": bytes.fromhex(contract_address[2:]),  # Store as bytes with '0x' prefix
        "contractCreatorAddress": bytes.fromhex(creator_address[2:]),
        "contractCode": bytes.fromhex(contract_code[2:]),
        "creationTransaction": transfer_doc['txHash'],
        "status": receipt.get('status', '0x1'),  # Default to success if status not present
        "isToken": is_token,
        "tokenName": name if is_token else "",
        "tokenSymbol": symbol if is_token else "",
        "tokenDecimals": decimals if is_token else 0,
        "updatedAt": datetime.utcnow().isoformat()
    }
    
    # Insert or update contract
    contracts_collection.update_one(
        {"contractAddress": contract_doc["contractAddress"]},
        {"$set": contract_doc},
        upsert=True
    )
    
    return True

def main():
    # Connect to MongoDB
    client = MongoClient(MONGO_URI)
    db = client['qrldata-b2h']  # Connect to correct database
    transfers_collection = db.transfer
    contracts_collection = db.contractCode
    
    # Debug: Print database and collection names
    print("Available databases:", client.list_database_names())
    print("Available collections:", db.list_collection_names())
    
    # Debug: Print a few sample documents
    print("\nSample documents from transfer collection:")
    sample_docs = transfers_collection.find({"contractAddress": {"$exists": True}}).limit(3)
    for doc in sample_docs:
        print("\nDocument:")
        for key, value in doc.items():
            if isinstance(value, bytes):
                print(f"{key}: 0x{value.hex()}")
            else:
                print(f"{key}: {value}")
    
    # Find all documents that have a contractAddress field
    query = {"contractAddress": {"$exists": True}}
    contract_txs = transfers_collection.find(query)
    total_txs = transfers_collection.count_documents(query)
    
    print(f"\nFound {total_txs} contract creation transactions to process")
    
    if total_txs == 0:
        print("No transactions found. Exiting.")
        return
        
    processed = 0
    successful = 0
    
    for tx in contract_txs:
        processed += 1
        print(f"\nProcessing transaction {processed}/{total_txs}")
        
        # Get the contract address directly from the document
        contract_address = tx['contractAddress']
        creator_address = tx['from']
        
        print(f"Raw contract address: {contract_address}")
        print(f"Raw creator address: {creator_address}")
        
        # Get contract code
        contract_code = get_contract_code(contract_address)
        if not contract_code or contract_code == "0x":
            print(f"No code found for contract {contract_address}")
            continue
        
        # Get token information
        name, symbol, decimals, is_token = get_token_info(contract_address)
        if is_token:
            print(f"Found token contract: Name={name}, Symbol={symbol}, Decimals={decimals}")
        
        # Create contract document
        contract_doc = {
            "contractAddress": contract_address,  # Keep original format
            "contractCreatorAddress": creator_address,  # Keep original format
            "contractCode": contract_code,  # Keep as hex string
            "creationTransaction": tx['txHash'],
            "status": tx.get('status', '0x1'),
            "isToken": is_token,
            "tokenName": name if is_token else "",
            "tokenSymbol": symbol if is_token else "",
            "tokenDecimals": decimals if is_token else 0,
            "updatedAt": datetime.utcnow().isoformat()
        }
        
        # Insert or update contract
        contracts_collection.update_one(
            {"contractAddress": contract_doc["contractAddress"]},
            {"$set": contract_doc},
            upsert=True
        )
        
        successful += 1
            
        if processed % 10 == 0:
            print(f"Progress: {processed}/{total_txs} transactions processed, {successful} contracts created")
        
        # Small delay to avoid overwhelming the node
        time.sleep(0.1)
    
    print(f"\nReindexing complete!")
    print(f"Total transactions processed: {processed}")
    print(f"Total contracts created: {successful}")

if __name__ == "__main__":
    main() 