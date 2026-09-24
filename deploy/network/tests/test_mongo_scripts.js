const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

function run(file, context, secrets = {}) {
  const source = fs.readFileSync(path.join(__dirname, '..', file), 'utf8');
  vm.runInNewContext(source, {
    require: () => ({ readFileSync: (name) => secrets[path.basename(name)] }),
    print: () => {},
    ...context,
  });
}

const credentials = {
  admin_user: 'fixture-admin',
  admin_password: 'fixture-admin-password',
  app_user: 'fixture-app',
  app_password: 'fixture-app-password',
};

test('application user has write access only to the new v3 database', () => {
  let request;
  run('mongo-app-user.js', {
    db: { getSiblingDB: (database) => {
      assert.equal(database, 'qrldata-v3');
      return { createUser: (value) => { request = value; } };
    } },
  }, credentials);
  assert.equal(request.user, 'fixture-app');
  assert.equal(JSON.stringify(request.roles), '[{"role":"readWrite","db":"qrldata-v3"}]');
});

test('shared administrator credentials cannot initialize the application user', () => {
  assert.throws(() => run('mongo-app-user.js', {}, {
    ...credentials, app_password: credentials.admin_password,
  }), /separate from administrator/);
});

test('replica initialization uses only its new dedicated identity', () => {
  const requests = [];
  run('mongo-init-replica.js', { Mongo: function () {
    return { getDB: () => ({ runCommand: (request) => {
      requests.push(request);
      return requests.length === 1 ? { ok: 0, code: 94 } : { ok: 1 };
    } }) };
  } }, credentials);
  assert.equal(requests[1].replSetInitiate._id, 'zondscan-v3');
  assert.equal(requests[1].replSetInitiate.members[0].host, 'localhost:27017');
});

test('an existing foreign replica set is rejected without changing it', () => {
  let requests = 0;
  assert.throws(() => run('mongo-init-replica.js', { Mongo: function () {
    return { getDB: () => ({ runCommand: () => {
      requests += 1;
      return { ok: 1, set: 'v2' };
    } }) };
  } }, credentials), /unexpected identity/);
  assert.equal(requests, 1);
});
