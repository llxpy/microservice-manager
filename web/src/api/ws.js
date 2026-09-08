let ws = null
let retryTimer = null
const listeners = new Set()
let connected = false

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws`)
  ws.onopen = () => {
    connected = true
    send({ action: 'subscribe', serviceId: '*' })
    listeners.forEach((fn) => fn({ type: '_ws', data: 'open' }))
  }
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      listeners.forEach((fn) => fn(msg))
    } catch (e) { /* ignore */ }
  }
  ws.onclose = () => {
    connected = false
    listeners.forEach((fn) => fn({ type: '_ws', data: 'close' }))
    clearTimeout(retryTimer)
    retryTimer = setTimeout(connect, 3000)
  }
  ws.onerror = () => ws.close()
}

export function initWS() {
  if (!ws) connect()
}

export function onMessage(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export function send(obj) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(obj))
  }
}

export function isConnected() {
  return connected
}

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined
  })
  return res.json()
}

export const rest = {
  list: () => api('GET', '/api/services'),
  get: (id) => api('GET', `/api/services/${encodeURIComponent(id)}`),
  start: (id) => api('POST', `/api/services/${encodeURIComponent(id)}/start`),
  stop: (id) => api('POST', `/api/services/${encodeURIComponent(id)}/stop`),
  restart: (id) => api('POST', `/api/services/${encodeURIComponent(id)}/restart`),
  groupStart: (g) => api('POST', `/api/groups/${encodeURIComponent(g)}/start`),
  groupStop: (g) => api('POST', `/api/groups/${encodeURIComponent(g)}/stop`),
  scan: (dir) => api('GET', '/api/discovery/scan' + (dir ? `?dir=${encodeURIComponent(dir)}` : '')),
  getSettings: () => api('GET', '/api/settings'),
  saveSettings: (body) => api('POST', '/api/settings', body),
  logs: (id, lines = 200) => api('GET', `/api/logs/${encodeURIComponent(id)}?lines=${lines}`)
}
