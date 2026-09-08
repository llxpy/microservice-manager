<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">⚙️ 微服务轻量管家</div>
      <div class="actions">
        <span class="ws-dot" :class="online ? 'on' : 'off'" :title="online ? '已连接' : '重连中'"></span>
        <button class="btn" @click="startAll">全部启动</button>
        <button class="btn" @click="stopAll">全部停止</button>
        <button class="btn" @click="rescan">{{ scanning ? '扫描中…' : '重新扫描' }}</button>
      </div>
    </header>
    <main class="main">
      <ServiceList
        :services="services"
        @open-log="openLog"
        @open-metrics="openMetrics"
        @refresh="refresh"
      />
    </main>

    <div v-if="logService" class="drawer-mask" @click.self="logService = null">
      <div class="drawer">
        <div class="drawer-head">
          <span>日志 — {{ logService }}</span>
          <div>
            <button class="btn small" @click="clearTerm">清屏</button>
            <button class="btn small" @click="logService = null">关闭</button>
          </div>
        </div>
        <LogTerminal :service-id="logService" ref="terminal" />
      </div>
    </div>

    <div v-if="metricService" class="drawer-mask" @click.self="metricService = null">
      <div class="dialog">
        <div class="drawer-head">
          <span>资源指标 — {{ metricService }}</span>
          <button class="btn small" @click="metricService = null">关闭</button>
        </div>
        <MetricsChart :service-id="metricService" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import ServiceList from './views/ServiceList.vue'
import LogTerminal from './components/LogTerminal.vue'
import MetricsChart from './components/MetricsChart.vue'
import { initWS, onMessage, rest } from './api/ws.js'

const services = ref([])
const online = ref(false)
const scanning = ref(false)
const logService = ref(null)
const metricService = ref(null)
const terminal = ref(null)
let off = null

function refresh() {
  rest.list().then((list) => {
    services.value = list || []
  })
}

function startAll() {
  const groups = [...new Set(services.value.map((s) => s.group))]
  groups.forEach((g) => rest.groupStart(g))
}
function stopAll() {
  const groups = [...new Set(services.value.map((s) => s.group))]
  groups.forEach((g) => rest.groupStop(g))
}
async function rescan() {
  scanning.value = true
  try { await rest.scan() } finally { scanning.value = false }
  refresh()
}
function openLog(id) { logService.value = id }
function openMetrics(id) { metricService.value = id }
function clearTerm() { terminal.value && terminal.value.clear() }

onMounted(() => {
  initWS()
  off = onMessage((msg) => {
    if (msg.type === '_ws') {
      online.value = msg.data === 'open'
      return
    }
    if (msg.type === 'list') {
      services.value = msg.services || []
    } else if (msg.type === 'status') {
      const i = services.value.findIndex((s) => s.id === msg.serviceId)
      if (i >= 0) services.value[i] = { ...services.value[i], ...msg.status }
      refresh()
    }
  })
  refresh()
})
onUnmounted(() => off && off())
</script>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: #10141c; color: #d7dde7; font-family: 'Segoe UI', system-ui, sans-serif; font-size: 14px; }
.app { min-height: 100vh; display: flex; flex-direction: column; }
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; background: #171d29; border-bottom: 1px solid #232c3d;
  position: sticky; top: 0; z-index: 10;
}
.brand { font-size: 17px; font-weight: 600; }
.actions { display: flex; align-items: center; gap: 8px; }
.btn {
  background: #24304a; color: #d7dde7; border: 1px solid #33415e;
  border-radius: 6px; padding: 5px 12px; cursor: pointer; font-size: 13px;
}
.btn:hover { background: #2e3d5e; }
.btn.small { padding: 3px 9px; font-size: 12px; }
.btn.primary { background: #2563eb; border-color: #2563eb; }
.btn.danger { background: #7f1d1d; border-color: #991b1b; }
.btn:disabled { opacity: .5; cursor: not-allowed; }
.ws-dot { width: 9px; height: 9px; border-radius: 50%; display: inline-block; margin-right: 6px; }
.ws-dot.on { background: #22c55e; }
.ws-dot.off { background: #ef4444; }
.main { flex: 1; padding: 16px 20px; }
.drawer-mask {
  position: fixed; inset: 0; background: rgba(0,0,0,.55); z-index: 100;
  display: flex; align-items: center; justify-content: center;
}
.drawer {
  width: min(900px, 92vw); height: 80vh; background: #0d1117;
  border: 1px solid #2a3550; border-radius: 10px; display: flex; flex-direction: column; overflow: hidden;
}
.dialog {
  width: min(860px, 92vw); background: #0d1117;
  border: 1px solid #2a3550; border-radius: 10px; display: flex; flex-direction: column; overflow: hidden;
}
.drawer-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px; background: #171d29; border-bottom: 1px solid #232c3d;
}
</style>
