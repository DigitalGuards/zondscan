// The official MongoDB image runs this once while initializing an empty volume.
const fs = require('fs');
const appUser = fs.readFileSync('/run/mongodb/app_user', 'utf8').trim();
const appPassword = fs.readFileSync('/run/mongodb/app_password', 'utf8').trim();
const adminUser = fs.readFileSync('/run/mongodb/admin_user', 'utf8').trim();
const adminPassword = fs.readFileSync('/run/mongodb/admin_password', 'utf8').trim();
if (!appUser || !appPassword || appUser === adminUser || appPassword === adminPassword) {
  throw new Error('V3 application credentials must be nonempty and separate from administrator credentials');
}
db.getSiblingDB('qrldata-v3').createUser({
  user: appUser,
  pwd: appPassword,
  roles: [{role: 'readWrite', db: 'qrldata-v3'}],
});
