const { AsyncLocalStorage } = require('async_hooks');

const storage = new AsyncLocalStorage();

function run(context, callback) {
  return storage.run(context || null, callback);
}

function get() {
  return storage.getStore() || null;
}

module.exports = { run, get };
