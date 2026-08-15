const { expect } = require('chai');
const seedData = require('../lib/seed_data');
const seedFixture = require('../scripts/seed_synthetic_rdi_fixture');

describe('synthetic-rdi fixture opt-in contract', function () {
  const envNames = [
    'AETHERLINK_RDI_FIXTURE_MODE',
    'AETHERLINK_RDI_FIXTURE_PID',
    'SYNTHETIC_RDI_PID',
    'AETHERLINK_DB_HOST',
    'AETHERLINK_DB_PORT',
    'AETHERLINK_DB_NAME',
    'AETHERLINK_DB_USER',
    'AETHERLINK_DB_PASSWORD',
    'GOTP_DB_PSQL_HOST',
    'GOTP_DB_PSQL_PORT',
    'GOTP_DB_PSQL_DBNAME',
    'GOTP_DB_PSQL_USERNAME',
    'GOTP_DB_PSQL_PASSWORD',
    'PGPASSWORD',
    'AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT',
    'AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES'
  ];
  const originalEnv = {};

  beforeEach(function () {
    envNames.forEach(name => { originalEnv[name] = process.env[name]; delete process.env[name]; });
  });

  afterEach(function () {
    envNames.forEach(name => {
      if (originalEnv[name] === undefined) delete process.env[name];
      else process.env[name] = originalEnv[name];
    });
  });

  function setDatabaseEnv(database, allowlist) {
    process.env.AETHERLINK_DB_HOST = '127.0.0.1';
    process.env.AETHERLINK_DB_PORT = '5432';
    process.env.AETHERLINK_DB_NAME = database;
    process.env.AETHERLINK_DB_USER = 'postgres';
    process.env.AETHERLINK_SYNTHETIC_RDI_ALLOWED_PORT = '5432';
    if (allowlist === undefined) delete process.env.AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES;
    else process.env.AETHERLINK_SYNTHETIC_RDI_ALLOWED_DATABASES = allowlist;
  }

  it('does not add a synthetic PID during ordinary real-RDI runs', function () {
    expect(seedData.getOptInSyntheticRdiPid()).to.equal('');
    expect(seedData.getRdiCandidatePids()).not.to.include('SYNTHRDI0001');
  });

  it('requires an explicit PID when synthetic-rdi mode is enabled', function () {
    process.env.AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi';
    expect(() => seedData.getOptInSyntheticRdiPid()).to.throw(/requires .*PID/i);
  });

  it('adds only a validated opt-in PID to the activation candidates', function () {
    process.env.AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi';
    process.env.AETHERLINK_RDI_FIXTURE_PID = 'synthrdi0001';
    expect(seedData.getOptInSyntheticRdiPid()).to.equal('SYNTHRDI0001');
    expect(seedData.getRdiCandidatePids()).to.include('SYNTHRDI0001');
  });

  it('binds the activated_pid assertion to the same explicit synthetic PID', function () {
    process.env.AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi';
    process.env.AETHERLINK_RDI_FIXTURE_PID = 'synthrdi0001';
    const testData = require('../lib/test_data');
    expect(testData.getDevicePID('activated_pid')).to.equal('SYNTHRDI0001');
  });

  it('rejects a PID that cannot satisfy the backend activation contract', function () {
    process.env.AETHERLINK_RDI_FIXTURE_MODE = 'synthetic-rdi';
    process.env.AETHERLINK_RDI_FIXTURE_PID = 'ordinary-device-id';
    expect(() => seedData.getOptInSyntheticRdiPid()).to.throw(/exactly 12 alphanumeric/i);
  });

  it('requires the predeploy database to be explicitly listed', function () {
    setDatabaseEnv('aetherlink_iot_predeploy_retest_20260814_r9d_synthetic');
    expect(() => seedFixture.getDatabaseOptions()).to.throw(/exact local test database name/i);
  });

  it('accepts an exact database entry and trims comma-separated allowlist values', function () {
    setDatabaseEnv(
      'aetherlink_iot_predeploy_retest_20260814_r9d_synthetic',
      ' other_db, aetherlink_iot_predeploy_retest_20260814_r9d_synthetic , another_db '
    );
    expect(seedFixture.getDatabaseOptions()).to.include({
      host: '127.0.0.1',
      port: '5432',
      database: 'aetherlink_iot_predeploy_retest_20260814_r9d_synthetic',
      user: 'postgres'
    });
  });

  it('does not let legacy local/test/isolated names bypass the explicit allowlist', function () {
    setDatabaseEnv('aetherlink_iot_local');
    expect(() => seedFixture.getDatabaseOptions()).to.throw(/exact local test database name/i);
  });
});
