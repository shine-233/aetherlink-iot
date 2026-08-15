/**
 * Resolve the MQTT endpoint used by the automation harness.
 *
 * Standard AetherLink deployments use 127.0.0.1:1883.  The checked-in
 * local-dev status configuration uses the `localdev-status` profile on port
 * 1885, so the profile is explicit instead of silently guessing a port from
 * whichever broker happens to be listening.
 */

const DEFAULT_SERVER = '127.0.0.1';
const DEFAULT_PORT = 1883;
const LOCALDEV_STATUS_PORT = 1885;

function parsePort(value, fallback) {
  if (value === undefined || value === null || String(value).trim() === '') {
    return fallback;
  }

  const port = Number(value);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`AUTOMATION_MQTT_PORT must be an integer between 1 and 65535, got ${value}`);
  }
  return port;
}

function parseAccessAddress(value, sourceName) {
  if (value === undefined || value === null || String(value).trim() === '') {
    return null;
  }

  const raw = String(value).trim();
  const urlValue = /^[a-z][a-z\d+.-]*:\/\//i.test(raw) ? raw : `mqtt://${raw}`;

  let parsed;
  try {
    parsed = new URL(urlValue);
  } catch (error) {
    throw new Error(`${sourceName} must be a valid MQTT host[:port] address, got ${value}`);
  }

  if (!parsed.hostname) {
    throw new Error(`${sourceName} must include a MQTT host, got ${value}`);
  }

  return {
    server: parsed.hostname,
    port: parsed.port ? parsePort(parsed.port, DEFAULT_PORT) : undefined
  };
}

function getMqttEndpoint(env = process.env) {
  const profile = String(env.AUTOMATION_MQTT_PROFILE || '').trim().toLowerCase();
  const profilePort = profile === 'localdev-status' ? LOCALDEV_STATUS_PORT : DEFAULT_PORT;
  const accessAddress =
    env.AUTOMATION_MQTT_ACCESS_ADDRESS ||
    env.AETHERLINK_MQTT_ACCESS_ADDRESS ||
    env.GOTP_MQTT_ACCESS_ADDRESS;
  const parsedAccessAddress = parseAccessAddress(
    accessAddress,
    accessAddress === env.GOTP_MQTT_ACCESS_ADDRESS
      ? 'GOTP_MQTT_ACCESS_ADDRESS'
      : accessAddress === env.AETHERLINK_MQTT_ACCESS_ADDRESS
        ? 'AETHERLINK_MQTT_ACCESS_ADDRESS'
        : 'AUTOMATION_MQTT_ACCESS_ADDRESS'
  );
  const server = String(
    env.AUTOMATION_MQTT_SERVER ||
      env.AETHERLINK_MQTT_SERVER ||
      parsedAccessAddress?.server ||
      DEFAULT_SERVER
  ).trim();
  const port = parsePort(
    env.AUTOMATION_MQTT_PORT || env.AETHERLINK_MQTT_PORT,
    parsedAccessAddress?.port || profilePort
  );

  if (!server) {
    throw new Error('AUTOMATION_MQTT_SERVER must not be empty');
  }

  return { server, port, profile: profile || 'standard' };
}

function mqttEndpointDescription(env = process.env) {
  const endpoint = getMqttEndpoint(env);
  return `${endpoint.server}:${endpoint.port}`;
}

module.exports = {
  DEFAULT_SERVER,
  DEFAULT_PORT,
  LOCALDEV_STATUS_PORT,
  parseAccessAddress,
  getMqttEndpoint,
  mqttEndpointDescription
};
