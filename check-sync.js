try {
    // Connect to MongoDB
    const db = connect('mongodb://localhost:27017/qrldata');
    
    // Get collection stats
    const blockCount = db.blocks.countDocuments();
    const txCount = db.transactionByAddress.countDocuments();
    const walletCount = db.addresses.countDocuments();
    
    // Get the latest block
    const latestBlock = db.blocks.find().sort({_id:-1}).limit(1).toArray();
    let currentBlock = 'Unknown';
    
    if (latestBlock.length > 0) {
        if (latestBlock[0].result && latestBlock[0].result.number) {
            currentBlock = latestBlock[0].result.number.toString();
        }
    }
    
    print("\nQRL Explorer Sync Status:");
    print("------------------------");
    print("Latest block in MongoDB: " + currentBlock);
    print("Total blocks synced:     " + blockCount);
    print("Total transactions:      " + txCount);
    print("Total wallets:           " + walletCount);
    print("------------------------");
    
    if (blockCount === 0) {
        print("\nStatus: Not started or no blocks synced yet");
    } else if (blockCount > 0) {
        if (txCount === 0) {
            print("\nStatus: Initial block sync in progress");
        } else {
            print("\nStatus: Synchronization active");
        }
        print("Note: The synchronizer processes new blocks every 60 seconds");
        print("      and updates market data every 5 minutes");
    }
} catch (error) {
    print("\nError checking sync status:");
    print(error);
    print("\nMake sure MongoDB is running and accessible at localhost:27017");
}
