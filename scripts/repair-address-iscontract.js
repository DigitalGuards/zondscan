// repair-address-iscontract.js
//
// One-time repair for isContract poisoning in the addresses collection.
// A syncer bug passed the recipient's isContract flag to BOTH parties of a
// transaction, so any EOA that ever sent a tx to a contract was stored with
// a sticky isContract=true row. Run this ONCE against the explorer MongoDB
// AFTER deploying the syncer fix (per-role upserts in db/transactions.go);
// running it against an unfixed syncer just lets the poisoning creep back.
//
// What it does:
//   1. Resets isContract to false on ALL addresses docs (including docs that
//      are missing the field entirely, so wallet counts see them too).
//   2. Re-flags isContract=true for every address that has a contractCode
//      document with actual deployed code (contractCode not "" and not "0x").
//
// Usage: mongosh <connection-string> scripts/repair-address-iscontract.js

const dbz = db.getSiblingDB('qrldata-z');

// --- Before summary ---
const totalAddresses = dbz.addresses.countDocuments({});
const flaggedBefore = dbz.addresses.countDocuments({ isContract: true });
const missingBefore = dbz.addresses.countDocuments({ isContract: { $exists: false } });
print(`addresses total:                 ${totalAddresses}`);
print(`isContract=true before repair:   ${flaggedBefore}`);
print(`isContract missing before:       ${missingBefore}`);

// --- Step 1: reset every addresses doc to isContract=false ---
const resetResult = dbz.addresses.updateMany({}, { $set: { isContract: false } });
print(`step 1: reset ${resetResult.modifiedCount} docs to isContract=false`);

// --- Step 2: re-flag real contracts from the contractCode collection ---
// Both collections store canonical Q-prefixed lowercase addresses
// (addresses.id == contractCode.address).
let contractsSeen = 0;
let flaggedNow = 0;
let missingAddressDoc = 0;
dbz.contractCode
  .find({ contractCode: { $exists: true, $nin: ['', '0x'] } }, { address: 1 })
  .forEach((contract) => {
    contractsSeen += 1;
    const res = dbz.addresses.updateOne(
      { id: contract.address },
      { $set: { isContract: true } }
    );
    if (res.matchedCount === 0) {
      // Contract has no addresses row yet (never held a balance); the fixed
      // syncer creates it on the next touch, nothing to repair here.
      missingAddressDoc += 1;
    } else {
      flaggedNow += 1;
    }
  });
print(`step 2: ${contractsSeen} contractCode docs with deployed code`);
print(`step 2: flagged ${flaggedNow} addresses docs isContract=true`);
print(`step 2: ${missingAddressDoc} contracts had no addresses doc (skipped)`);

// --- After summary ---
const flaggedAfter = dbz.addresses.countDocuments({ isContract: true });
const walletsAfter = dbz.addresses.countDocuments({ isContract: false });
print(`isContract=true after repair:    ${flaggedAfter} (was ${flaggedBefore})`);
print(`isContract=false after repair:   ${walletsAfter}`);
