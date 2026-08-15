const { expect } = require('chai');
const { resolveOtaSeedDatabaseOptions } = require('../lib/seed_data');

describe('OTA seed database target contract', function() {
  it('keeps the legacy local fallback outside strict mode', function() {
    expect(resolveOtaSeedDatabaseOptions({})).to.include({
      host: '127.0.0.1',
      port: '5432',
      user: 'postgres',
      database: 'aetherlink_iot_local'
    });
  });

  it('requires an explicit database target in strict mode', function() {
    expect(() => resolveOtaSeedDatabaseOptions({ AETHERLINK_STRICT_DB_TARGET: '1' }))
      .to.throw(/AETHERLINK_STRICT_DB_TARGET=1 requires an explicit database target/);
  });

  it('rejects mismatched backend and psql database targets in strict mode', function() {
    expect(() => resolveOtaSeedDatabaseOptions({
      AETHERLINK_STRICT_DB_TARGET: '1',
      AETHERLINK_DB_NAME: 'aetherlink_iot_restore_drill_20260814',
      GOTP_DB_PSQL_DBNAME: 'aetherlink_iot_local'
    })).to.throw(/AETHERLINK_DB_NAME and GOTP_DB_PSQL_DBNAME to match/);
  });

  it('resolves one explicit target for both backend aliases', function() {
    expect(resolveOtaSeedDatabaseOptions({
      AETHERLINK_STRICT_DB_TARGET: '1',
      AETHERLINK_DB_HOST: '127.0.0.1',
      AETHERLINK_DB_PORT: '5432',
      AETHERLINK_DB_NAME: 'aetherlink_iot_restore_drill_20260814',
      GOTP_DB_PSQL_DBNAME: 'aetherlink_iot_restore_drill_20260814',
      AETHERLINK_DB_USER: 'postgres',
      AETHERLINK_DB_PASSWORD: 'secret-that-must-not-be-logged'
    })).to.include({
      database: 'aetherlink_iot_restore_drill_20260814',
      password: 'secret-that-must-not-be-logged'
    });
  });

  it('accepts PGDATABASE as an explicit strict target', function() {
    expect(resolveOtaSeedDatabaseOptions({
      AETHERLINK_STRICT_DB_TARGET: '1',
      PGDATABASE: 'aetherlink_iot_predeploy_retest_20260814_r9d_synthetic'
    }).database).to.equal('aetherlink_iot_predeploy_retest_20260814_r9d_synthetic');
  });
});
