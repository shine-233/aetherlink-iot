import type { DeviceAccessGuideCommand } from './device-access-guide-state'

const shellQuote = (value: string) => value.replace(/'/g, "'\"'\"'")

const cString = (value: string) => value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')

const commandOption = (flag: string, value: string) => (value && !value.startsWith('<') ? ` ${flag} "${value}"` : '')

export const buildMqttCommands = (options: {
  endpoint: string
  host: string
  port: string
  clientId: string
  username: string
  password: string
  reportTopic: string
  controlTopic: string
  payload: string
}): DeviceAccessGuideCommand[] => {
  const passwordOption = commandOption('-P', options.password)
  const usernameOption = commandOption('-u', options.username)
  const clientIdOption = commandOption('-i', options.clientId)
  const mqttScheme = options.endpoint.startsWith('mqtts://') || options.port === '8883' ? 'mqtts' : 'mqtt'
  const nodePasswordLine = options.password ? `,\n  password: '${options.password}'` : ''
  const pythonPasswordArg = options.password ? `, '${options.password}'` : ''
  const tlsPythonLine = mqttScheme === 'mqtts' ? '\nclient.tls_set()' : ''

  return [
    {
      titleKey: 'custom.device_details.accessGuideMosquitto',
      language: 'bash',
      code: `mosquitto_pub -h ${options.host} -p ${options.port}${clientIdOption}${usernameOption}${passwordOption} -t "${options.reportTopic}" -m '${shellQuote(options.payload)}'`
    },
    {
      titleKey: 'custom.device_details.accessGuideNode',
      language: 'javascript',
      code: `import mqtt from 'mqtt'

const client = mqtt.connect('${mqttScheme}://${options.host}:${options.port}', {
  clientId: '${options.clientId}',
  username: '${options.username}'${nodePasswordLine}
})

client.on('connect', () => {
  client.publish('${options.reportTopic}', ${JSON.stringify(options.payload)})
  client.subscribe('${options.controlTopic}')
})`
    },
    {
      titleKey: 'custom.device_details.accessGuidePython',
      language: 'python',
      code: `import paho.mqtt.client as mqtt

client = mqtt.Client()
client.username_pw_set('${options.username}'${pythonPasswordArg})${tlsPythonLine}
client.connect('${options.host}', ${options.port}, 60)
client.publish('${options.reportTopic}', '''${options.payload}''')
client.subscribe('${options.controlTopic}')
client.loop_forever()`
    },
    {
      titleKey: 'custom.device_details.accessGuideC',
      language: 'c',
      code: `/* Eclipse Paho C starter */
MQTTClient_connectOptions opts = MQTTClient_connectOptions_initializer;
opts.username = "${cString(options.username)}";
opts.password = "${cString(options.password)}";
MQTTClient_connect(client, &opts);
MQTTClient_publish(client, "${cString(options.reportTopic)}", ${options.payload.length}, "${cString(options.payload)}", 1, 0, NULL);`
    }
  ]
}

export const buildHttpCommands = (endpoint: string, username: string, payload: string): DeviceAccessGuideCommand[] => {
  const authHeader = username && !username.startsWith('<') ? ` -H "Authorization: Bearer ${username}"` : ''
  const nodeAuthHeader = username && !username.startsWith('<') ? `,\n    authorization: 'Bearer ${username}'` : ''
  const pythonAuthHeader =
    username && !username.startsWith('<') ? `\nheaders['authorization'] = 'Bearer ${username}'` : ''

  return [
    {
      titleKey: 'custom.device_details.accessGuideCurl',
      language: 'bash',
      code: `curl -X POST "${endpoint}" -H "Content-Type: application/json"${authHeader} -d '${shellQuote(payload)}'`
    },
    {
      titleKey: 'custom.device_details.accessGuideNode',
      language: 'javascript',
      code: `await fetch('${endpoint}', {
  method: 'POST',
  headers: {
    'content-type': 'application/json'${nodeAuthHeader}
  },
  body: ${JSON.stringify(payload)}
})`
    },
    {
      titleKey: 'custom.device_details.accessGuidePython',
      language: 'python',
      code: `import requests

headers = {'content-type': 'application/json'}${pythonAuthHeader}
requests.post('${endpoint}', headers=headers, data='''${payload}''')`
    },
    {
      titleKey: 'custom.device_details.accessGuideC',
      language: 'c',
      code: `/* libcurl starter */
curl_easy_setopt(curl, CURLOPT_URL, "${cString(endpoint)}");
curl_easy_setopt(curl, CURLOPT_POSTFIELDS, "${cString(payload)}");`
    }
  ]
}
