<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">⚙️ 微服务轻量管家</div>
      <div class="scan-box">
        <el-input
          v-model="scanDir"
          placeholder="输入扫描目录，如 D:\svc（递归扫描所有子目录）"
          clearable
          style="width: 340px"
          @keyup.enter="saveAndScan"
        >
          <template #prepend>扫描目录</template>
        </el-input>
        <el-button type="primary" :loading="scanning" @click="saveAndScan">保存并扫描</el-button>
      </div>
      <div class="actions">
        <el-tag :type="online ? 'success' : 'danger'" effect="dark" size="small" round>
          {{ online ? 'WS 已连接' : 'WS 重连中' }}
        </el-tag>
        <el-button @click="startAll">全部启动</el-button>
        <el-button type="danger" @click="stopAll">全部停止</el-button>
        <el-button :loading="scanning" @click="rescan(false)">重新扫描</el-button>
      </div>
    </header>

    <main class="main">
      <el-collapse v-model="openGroups" class="groups">
        <el-collapse-item v-for="g in groups" :key="g.name" :name="g.name">
          <template #title>
            <span class="group-title">
              分组：{{ g.name }}
              <el-tag size="small" type="info" round>{{ g.items.length }} 个服务</el-tag>
              <el-tag v-if="runningCount(g)" size="small" type="success" round>
                {{ runningCount(g) }} 运行中
              </el-tag>
            </span>
            <span class="group-ops" @click.stop>
              <el-button size="small" type="primary" @click="groupOp(g.name, 'start')">▶ 启动整组</el-button>
              <el-button size="small" type="danger" @click="groupOp(g.name, 'stop')">■ 停止整组</el-button>
            </span>
          </template>
          <ServiceTable
            :items="g.items"
            @open-log="logService = $event"
            @open-metrics="metricService = $event"
            @refresh="refresh"
          />
        </el-collapse-item>
      </el-collapse>
      <el-empty v-if="!groups.length" description="没有服务。请在上方输入 jar 所在目录并点击「保存并扫描」" />
    </main>

    <el-drawer
      v-model="logOpen"
      :title="`日志 — ${logService}`"
      size="55%"
      destroy-on-close
    >
      <LogTerminal v-if="logService" :service-id="logService" />
    </el-drawer>

    <el-dialog
      v-model="metricOpen"
      :title="`资源指标 — ${metricService}`"
      width="780px"
      destroy-on-close
    >
      <MetricsChart v-if="metricService" :service-id="metricService" />
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import ServiceTable from './views/ServiceTable.vue'
import LogTerminal from './components/LogTerminal.vue'
import MetricsChart from './components/MetricsChart.vue'
import { initWS, onMessage, rest } from './api/ws.js'

const services = ref([])
const online = ref(false)
const scanning = ref(false)
const scanDir = ref('')
const logService = ref(null)
const metricService = ref(null)
const openGroups = ref([])
const logOpen = computed({
  get: () => !!logService.value,
  set: (v) => { if (!v) logService.value = null }
})
const metricOpen = computed({
  get: () => !!metricService.value,
  set: (v) => { if (!v) metricService.value = null }
})
let off = null

const groups = computed(() => {
  const map = {}
  for (const s of services.value) {
    const g = s.group || 'default'
    ;(map[g] = map[g] || []).push(s)
  }
  const names = Object.keys(map).sort()
  openGroups.value = openGroups.value.length ? openGroups.value : names
  return names.map((name) => ({ name, items: map[name] }))
})

function runningCount(g) {
  return g.items.filter((s) => s.state === 'running' || s.state === 'starting').length
}

function refresh() {
  rest.list().then((list) => { services.value = list || [] })
}

function startAll() {
  groups.value.forEach((g) => rest.groupStart(g.name))
  ElMessage.success('已对全部分组发起启动')
}
function stopAll() {
  groups.value.forEach((g) => rest.groupStop(g.name))
  ElMessage.success('已对全部分组发起停止')
}
function groupOp(name, action) {
  action === 'start' ? rest.groupStart(name) : rest.groupStop(name)
  ElMessage.info(`分组 ${name} ${action === 'start' ? '启动' : '停止'}指令已发送`)
}
async function rescan(withDir = false) {
  scanning.value = true
  try {
    const res = withDir ? await rest.scan(scanDir.value) : await rest.scan()
    if (res && res.scanDir) scanDir.value = res.scanDir
    ElMessage.success(`扫描完成：新增 ${res.new} 个，已有 ${res.existing} 个`)
  } catch (e) {
    ElMessage.error('扫描失败')
  } finally {
    scanning.value = false
  }
  refresh()
}
async function saveAndScan() {
  if (!scanDir.value.trim()) {
    ElMessage.warning('请先输入扫描目录')
    return
  }
  scanning.value = true
  try {
    await rest.saveSettings({ scanDir: scanDir.value.trim() })
    const res = await rest.scan()
    ElMessage.success(`目录已保存，扫描完成：新增 ${res.new} 个，已有 ${res.existing} 个`)
  } catch (e) {
    ElMessage.error('操作失败')
  } finally {
    scanning.value = false
  }
  refresh()
}

onMounted(async () => {
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
    }
  })
  refresh()
  try {
    const s = await rest.getSettings()
    if (s && s.scanDir) scanDir.value = s.scanDir
  } catch (e) { /* ignore */ }
})
onUnmounted(() => off && off())
</script>

<style>
body { margin: 0; background: var(--el-bg-color-page); }
.app { min-height: 100vh; }
.topbar {
  display: flex; align-items: center; justify-content: space-between; gap: 16px;
  padding: 10px 20px; background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-lighter);
  position: sticky; top: 0; z-index: 10; flex-wrap: wrap;
}
.brand { font-size: 17px; font-weight: 700; white-space: nowrap; }
.scan-box { display: flex; gap: 8px; align-items: center; flex: 1; min-width: 420px; }
.actions { display: flex; align-items: center; gap: 8px; }
.main { padding: 16px 20px; }
.group-title { display: inline-flex; gap: 8px; align-items: center; font-weight: 600; }
.group-ops { margin-left: auto; margin-right: 16px; }
.groups { --el-collapse-border-color: var(--el-border-color-lighter); }
</style>
