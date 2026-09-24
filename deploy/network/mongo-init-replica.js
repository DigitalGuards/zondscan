// Run explicitly after initial user creation. No explorer or syncer is started.
const fs = require('fs');
const user = encodeURIComponent(fs.readFileSync('/run/mongodb/admin_user', 'utf8').trim());
const password = encodeURIComponent(fs.readFileSync('/run/mongodb/admin_password', 'utf8').trim());
const admin = new Mongo(`mongodb://${user}:${password}@127.0.0.1:27017/admin?directConnection=true`).getDB('admin');
const status = admin.runCommand({replSetGetStatus: 1});
if (status.ok) {
  if (status.set !== 'zondscan-v3') throw new Error('Existing replica set has an unexpected identity');
  print('V3 replica set already initialized');
} else if (status.code === 94) {
  const result = admin.runCommand({
    replSetInitiate: {_id: 'zondscan-v3', members: [{_id: 0, host: 'localhost:27017'}]},
  });
  if (!result.ok) throw new Error('V3 replica set initialization failed');
  print('V3 replica set initialized');
} else {
  throw new Error('V3 replica set status could not be verified');
}
