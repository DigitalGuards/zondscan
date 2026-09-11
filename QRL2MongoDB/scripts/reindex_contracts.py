#!/usr/bin/env python3

import hashlib
import os
from datetime import datetime
import requests
from dotenv import load_dotenv
import logging

if __package__:
    from .maintenance_lease import (
        MaintenanceInterrupted,
        MaintenanceLease,
        MaintenanceLeaseError,
        maintenance_shutdown_signals,
        require_mongo_uri,
    )
else:
    from maintenance_lease import (
        MaintenanceInterrupted,
        MaintenanceLease,
        MaintenanceLeaseError,
        maintenance_shutdown_signals,
        require_mongo_uri,
    )

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

NODE_URL = os.getenv('NODE_URL', 'https://qrlwallet.com/api/qrl-rpc/testnet')

ABI_WORD_BYTES = 64
ABI_WORD_HEX_LENGTH = ABI_WORD_BYTES * 2
UINT256_HEX_LENGTH = 64
ADDRESS_HEX_LENGTH = 128
MAX_DYNAMIC_STRING_BYTES = 1 << 20


def checksummed_body(lower_body):
    """Return wallet.js-compatible QIP-55 checksum casing for a hex body."""
    digest = hashlib.shake_256(lower_body.encode('ascii')).digest(ADDRESS_HEX_LENGTH // 2)
    result = []
    for index, character in enumerate(lower_body):
        if 'a' <= character <= 'f':
            byte = digest[index // 2]
            nibble = byte >> 4 if index % 2 == 0 else byte & 0x0f
            if nibble >= 8:
                character = character.upper()
        result.append(character)
    return ''.join(result)


def has_valid_case(body):
    """Accept uniform casing and validate checksum-bearing mixed casing."""
    has_lower = any('a' <= character <= 'f' for character in body)
    has_upper = any('A' <= character <= 'F' for character in body)
    if not has_lower or not has_upper:
        return True
    lower_body = body.lower()
    return body == checksummed_body(lower_body)


def address_hex(address):
    """Return a lowercase QIP-55 address body, or None for malformed input."""
    if isinstance(address, (bytes, bytearray)):
        return bytes(address).hex() if len(address) == ABI_WORD_BYTES else None
    if not isinstance(address, str):
        return None

    value = address.strip()
    if value.startswith(('Q', 'q')):
        value = value[1:]
    elif value.startswith(('0x', '0X')):
        value = value[2:]
    else:
        return None

    if len(value) != ADDRESS_HEX_LENGTH:
        return None
    try:
        int(value, 16)
    except ValueError:
        return None
    if not has_valid_case(value):
        return None
    return value.lower()


def normalize_address(address):
    """Convert a QIP-55 address to canonical Q-prefix lowercase form."""
    body = address_hex(address)
    if body is None:
        raise ValueError(f"Invalid QIP-55 address: {address}")
    return 'Q' + body


def decode_uint256_word(value):
    """Decode one exact 64-byte ABI word carrying a low-half uint256."""
    if not isinstance(value, str) or not value.startswith('0x'):
        raise ValueError("ABI uint256 word must have a 0x prefix")
    word = value[2:]
    if len(word) != ABI_WORD_HEX_LENGTH:
        raise ValueError(
            f"ABI uint256 word has {len(word)} hex characters, "
            f"expected {ABI_WORD_HEX_LENGTH}"
        )
    try:
        high = int(word[:-UINT256_HEX_LENGTH], 16)
        low = int(word[-UINT256_HEX_LENGTH:], 16)
    except ValueError as error:
        raise ValueError("ABI uint256 word is not hexadecimal") from error
    if high != 0:
        raise ValueError("ABI uint256 word has nonzero high bytes")
    return low


def decode_dynamic_string(value):
    """Decode a canonical single-string return using 64-byte ABI words."""
    if not isinstance(value, str) or not value.startswith('0x'):
        raise ValueError("ABI string result must have a 0x prefix")
    payload = value[2:]
    if len(payload) < 2 * ABI_WORD_HEX_LENGTH:
        raise ValueError("ABI dynamic string payload is too short")

    offset = decode_uint256_word('0x' + payload[:ABI_WORD_HEX_LENGTH])
    if offset != ABI_WORD_BYTES:
        raise ValueError(
            f"ABI dynamic string offset is {offset} bytes, expected {ABI_WORD_BYTES}"
        )

    length_start = offset * 2
    length_end = length_start + ABI_WORD_HEX_LENGTH
    if length_end > len(payload):
        raise ValueError("ABI dynamic string length word is out of bounds")
    length = decode_uint256_word('0x' + payload[length_start:length_end])
    if length > MAX_DYNAMIC_STRING_BYTES:
        raise ValueError(
            f"ABI dynamic string length {length} exceeds {MAX_DYNAMIC_STRING_BYTES}"
        )

    data_start = length_end
    data_end = data_start + length * 2
    padded_bytes = ((length + ABI_WORD_BYTES - 1) // ABI_WORD_BYTES) * ABI_WORD_BYTES
    padded_end = data_start + padded_bytes * 2
    if data_end > len(payload) or padded_end != len(payload):
        raise ValueError("ABI dynamic string data length does not match its payload")

    padding = payload[data_end:padded_end]
    if padding.strip('0'):
        raise ValueError("ABI dynamic string has nonzero padding")
    try:
        return bytes.fromhex(payload[data_start:data_end]).decode('utf-8')
    except (ValueError, UnicodeDecodeError) as error:
        raise ValueError("ABI dynamic string is not valid UTF-8 hex data") from error


def decode_token_string(value):
    """Decode a dynamic string or a single fixed 64-byte string word."""
    try:
        return decode_dynamic_string(value)
    except ValueError as dynamic_error:
        if not isinstance(value, str) or not value.startswith('0x'):
            raise dynamic_error
        payload = value[2:]
        if len(payload) != ABI_WORD_HEX_LENGTH:
            raise dynamic_error
        try:
            fixed = bytes.fromhex(payload)
        except ValueError as error:
            raise ValueError("ABI fixed string is not hexadecimal") from error
        fixed = fixed.rstrip(b'\x00')
        try:
            return fixed.decode('utf-8')
        except UnicodeDecodeError as error:
            raise ValueError("ABI fixed string is not valid UTF-8") from error

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
    return make_rpc_call("qrl_getCode", [address, "latest"])

def get_transaction_receipt(tx_hash):
    """Get transaction receipt from the node."""
    return make_rpc_call("qrl_getTransactionReceipt", [tx_hash])

def call_contract_method(contract_address, method_signature):
    """Call a contract method using eth_call."""
    data = {
        "to": contract_address,
        "data": method_signature
    }
    return make_rpc_call("qrl_call", [data, "latest"])

def get_token_info(contract_address):
    """Get ERC20 token information from a contract."""
    contract_address = normalize_address(contract_address)

    # Method signatures for ERC20 interface
    NAME_SIG = "0x06fdde03"      # name()
    SYMBOL_SIG = "0x95d89b41"    # symbol()
    DECIMALS_SIG = "0x313ce567"  # decimals()
    
    name = symbol = ""
    decimals = 0
    is_token = False
    
    # Try to get token name
    name_result = call_contract_method(contract_address, NAME_SIG)
    if name_result:
        try:
            name = decode_token_string(name_result).strip()
            if name:
                is_token = True
            logger.info(f"Decoded token name for {contract_address}: '{name}'")
        except Exception as e:
            logger.error(f"Error decoding name for {contract_address}: {e}")
    
    # Try to get token symbol
    symbol_result = call_contract_method(contract_address, SYMBOL_SIG)
    if symbol_result:
        try:
            symbol = decode_token_string(symbol_result).strip()
            if symbol:
                is_token = True
            logger.info(f"Decoded token symbol for {contract_address}: '{symbol}'")
        except Exception as e:
            logger.error(f"Error decoding symbol for {contract_address}: {e}")
    
    # Try to get decimals
    decimals_result = call_contract_method(contract_address, DECIMALS_SIG)
    if decimals_result:
        try:
            decoded_decimals = decode_uint256_word(decimals_result)
            if decoded_decimals > 255:
                raise ValueError(f"token decimals exceeds uint8: {decoded_decimals}")
            decimals = decoded_decimals
            is_token = True
        except Exception as e:
            logger.error(f"Error decoding decimals for {contract_address}: {e}")
    
    return name, symbol, decimals, is_token


def process_contract_creation(transfer_doc, contracts_collection, lease):
    """Process a contract creation transaction and store contract information."""
    # Get transaction hash in hex format
    tx_hash = "0x" + transfer_doc['txHash'].hex() if isinstance(transfer_doc['txHash'], bytes) else transfer_doc['txHash']
    
    # Get transaction receipt to find contract address
    receipt = get_transaction_receipt(tx_hash)
    if not receipt or not receipt.get('contractAddress'):
        logger.error(f"No contract address found for transaction {tx_hash}")
        return False

    try:
        contract_address = normalize_address(receipt['contractAddress'])
        creator_address = normalize_address(transfer_doc['from'])
    except (KeyError, ValueError) as error:
        logger.error(f"Invalid QIP-55 contract creation address for {tx_hash}: {error}")
        return False
    
    # Get contract code
    contract_code = get_contract_code(contract_address)
    if not contract_code or contract_code == "0x":
        logger.error(f"No code found for contract {contract_address}")
        return False
    
    # Get token information
    name, symbol, decimals, is_token = get_token_info(contract_address)
    
    # Create contract document
    contract_doc = {
        "address": contract_address,
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
        result = lease.guard_write(
            lambda: contracts_collection.update_one(
                {"address": contract_doc["address"]},
                {"$set": contract_doc},
                upsert=True
            )
        )
        logger.info(f"MongoDB update result - Matched: {result.matched_count}, Modified: {result.modified_count}, Upserted: {result.upserted_id is not None}")
    except MaintenanceLeaseError:
        raise
    except Exception as e:
        logger.error(f"Error updating contract in MongoDB: {e}")
        return False
    
    return True


def reindex_contracts(client, lease):
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
        lease.ensure_held()
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
            
            try:
                contract_address = normalize_address(contract_address)
                creator_address = normalize_address(creator_address)
            except ValueError as error:
                logger.error(f"Skipping malformed QIP-55 contract creation: {error}")
                continue
            
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
                result = lease.guard_write(
                    lambda: contracts_collection.update_one(
                        {"address": contract_doc["address"]},
                        {"$set": contract_doc},
                        upsert=True
                    )
                )
                logger.info(f"Contract {contract_address} updated - Matched: {result.matched_count}, Modified: {result.modified_count}, Upserted: {result.upserted_id is not None}")
                contracts_created += 1
            except MaintenanceLeaseError:
                raise
            except Exception as e:
                logger.error(f"Error updating contract in MongoDB: {e}")
                
        except MaintenanceLeaseError:
            raise
        except Exception as e:
            logger.error(f"Error processing transfer {i}/{total_transfers}: {str(e)}", exc_info=True)
    
    logger.info("\nReindexing complete!")
    logger.info(f"Total transactions processed: {total_transfers}")
    logger.info(f"Total contracts created: {contracts_created}")


def main():
    from pymongo import MongoClient

    mongo_uri = require_mongo_uri()
    logger.info("Connecting to the configured MongoDB replica set")
    client = MongoClient(
        mongo_uri,
        serverSelectionTimeoutMS=5_000,
        connectTimeoutMS=5_000,
        socketTimeoutMS=10_000,
    )
    try:
        with maintenance_shutdown_signals():
            with MaintenanceLease(client, "reindex-contracts") as lease:
                reindex_contracts(client, lease)
    finally:
        client.close()


if __name__ == "__main__":
    try:
        main()
    except MaintenanceInterrupted as error:
        logger.warning("Contract reindex interrupted: %s", error)
        raise SystemExit(130) from error
    except MaintenanceLeaseError as error:
        logger.critical("Contract reindex stopped by lease enforcement: %s", error)
        raise SystemExit(1) from error
