<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <div class="brand-icon">⚙️</div>
        <div>
          <div class="brand-title">微服务轻量管家</div>
          <div class="brand-sub">Java Services Manager</div>
        </div>
      </div>
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
        <el-tooltip :content="online ? 'WebSocket 已连接' : 'WebSocket 重连中'" placement="bottom">
          <span class="ws-dot" :class="online ? 'on' : 'off'"></span>
        </el-tooltip>
        <el-button @click="startAll">全部启动</el-button>
        <el-button type="danger" plain @click="stopAll">全部停止</el-button>
        <el-button type="success" plain :disabled="buildRunning" @click="buildOpen = true">
          {{ buildRunning ? '构建中…' : '🔨 构建项目' }}
        </el-button>
        <el-button type="warning" plain @click="addOpen = true">➕ 添加服务</el-button>
        <el-button :loading="scanning" @click="rescan(false)">重新扫描</el-button>
      </div>
    </header>

    <main class="main">
      <el-collapse v-model="openGroups" class="groups">
        <el-collapse-item v-for="g in groups" :key="g.name" :name="g.name">
          <template #title>
            <span class="group-title">
              <el-tag effect="plain" round>{{ g.name }}</el-tag>
              <span class="count">{{ g.items.length }} 个服务</span>
              <el-tag v-if="runningCount(g)" size="small" type="success" effect="light" round>
                {{ runningCount(g) }} 运行中
              </el-tag>
            </span>
            <span class="group-ops" @click.stop>
              <el-button size="small" type="primary" plain @click="groupOp(g.name, 'start')">▶ 启动整组</el-button>
              <el-button size="small" type="danger" plain @click="groupOp(g.name, 'stop')">■ 停止整组</el-button>
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
      :title="`日志 — ${nameOf(logService)}`"
      size="55%"
      destroy-on-close
    >
      <LogTerminal v-if="logService" :service-id="logService" />
    </el-drawer>

    <el-dialog
      v-model="metricOpen"
      :title="`资源指标 — ${nameOf(metricService)}`"
      width="780px"
      destroy-on-close
    >
      <MetricsChart v-if="metricService" :service-id="metricService" />
    </el-dialog>

    <el-dialog v-model="addOpen" title="添加服务（任意语言）" width="560px">
      <el-form label-width="90px">
        <el-form-item label="名称" required><el-input v-model="addForm.name" placeholder="my-service" /></el-form-item>
        <el-form-item label="启动命令" required>
          <el-input v-model="addForm.command" placeholder="python app.py / node server.js / go run main.go ..." />
        </el-form-item>
        <el-form-item label="工作目录"><el-input v-model="addForm.workDir" placeholder="命令执行所在目录" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="addForm.port" :min="0" :max="65535" controls-position="right" /></el-form-item>
        <el-form-item label="分组"><el-input v-model="addForm.group" placeholder="default" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="addForm.description" type="textarea" :rows="2" placeholder="这个服务是干什么的" /></el-form-item>
        <el-form-item label="自动重启"><el-switch v-model="addForm.autoRestart" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addOpen = false">取消</el-button>
        <el-button type="primary" @click="doAdd">添加</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="buildOpen" title="构建项目（mvn package -DskipTests）" width="900px" destroy-on-close>
      <div class="build-bar">
        <el-input v-model="buildDir" placeholder="项目目录（含 pom.xml），如 D:\Java\itheima-chain-cloud" clearable @keyup.enter="doBuild" />
        <el-button v-if="!buildRunning" type="success" @click="doBuild">开始构建</el-button>
        <el-button v-else type="danger" @click="doBuildStop">停止</el-button>
      </div>
      <div class="build-term">
        <LogTerminal service-id="__build__" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onUnmounted } from 'vue'
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
const buildOpen = ref(false)
const buildRunning = ref(false)
const buildDir = ref('')
const addOpen = ref(false)
const addForm = reactive({ name: '', command: '', workDir: '', port: 0, group: '', description: '', autoRestart: true })

function doAdd() {
  if (!addForm.name.trim() || !addForm.command.trim()) {
    ElMessage.warning('名称和启动命令必填')
    return
  }
  rest.create({ ...addForm, name: addForm.name.trim(), command: addForm.command.trim() }).then((res) => {
    if (res && res.error) {
      ElMessage.error(res.error)
      return
    }
    ElMessage.success('已添加')
    addOpen.value = false
    Object.assign(addForm, { name: '', command: '', workDir: '', port: 0, group: '', description: '', autoRestart: true })
    refresh()
  })
}
let buildPoll = null

function doBuild() {
  if (!buildDir.value.trim()) {
    ElMessage.warning('请填写项目目录')
    return
  }
  rest.buildStart(buildDir.value.trim()).then((res) => {
    if (res && res.error) {
      ElMessage.error(res.error)
      return
    }
    buildRunning.value = true
  })
}
function doBuildStop() {
  rest.buildStop().then(() => ElMessage.info('已发送停止指令'))
}
function pollBuild() {
  rest.buildStatus().then((s) => {
    const was = buildRunning.value
    buildRunning.value = s.running
    if (buildOpen.value && s.dir && !buildDir.value) buildDir.value = s.dir
    if (was && !s.running) {
      ElMessage(s.lastExit === '构建成功' ? '构建完成，已自动扫描新服务' : `构建结束：${s.lastExit}`)
      refresh()
    }
  })
}
const logOpen = computed({
  get: () => !!logService.value,
  set: (v) => { if (!v) logService.value = null }
})
const metricOpen = computed({
  get: () => !!metricService.value,
  set: (v) => { if (!v) metricService.value = null }
})
function nameOf(id) {
  const s = services.value.find((x) => x.id === id)
  return s ? s.name : (id || '')
}
let off = null

const groups = computed(() => {
  const map = {}
  for (const s of services.value) {
    const g = s.group || 'default'
    ;(map[g] = map[g] || []).push(s)
  }
  const names = Object.keys(map).sort()
  return names.map((name) => ({ name, items: map[name] }))
})

// 首次有分组数据时默认全部展开（不能在 computed 里写状态，会导致无限渲染循环）
let groupsInited = false
watch(groups, (gs) => {
  if (!groupsInited && gs.length) {
    groupsInited = true
    openGroups.value = gs.map((g) => g.name)
  }
}, { immediate: true })

function runningCount(g) {
  return g.items.filter((s) => s.state === 'running' || s.state === 'starting').length
}
function refresh() {
  rest.list().then((list) => {
    if (list) mergeList(list)
  })
}

// 按行合并：数据没变化的行保留原引用，避免 el-table 整表重渲染闪烁
const STATIC_KEYS = ['name', 'group', 'path', 'port', 'description', 'command', 'type', 'javaOpts', 'workDir']
const RUNTIME_KEYS = ['state', 'pid', 'health', 'cpu', 'memMb', 'threads', 'uptime', 'port']
function mergeList(list) {
  const curMap = new Map(services.value.map((s) => [s.id, s]))
  services.value = list.map((item) => {
    const cur = curMap.get(item.id)
    if (!cur) return item
    const merged = { ...cur, ...item }
    const staticSame = STATIC_KEYS.every((k) => merged[k] === cur[k])
    if (!staticSame) return merged
    const runtimeSame = RUNTIME_KEYS.every((k) => merged[k] === cur[k])
    return runtimeSame ? cur : merged
  })
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
      mergeList(msg.services || [])
    } else if (msg.type === 'status') {
      const i = services.value.findIndex((s) => s.id === msg.serviceId)
      if (i >= 0) {
        const cur = services.value[i]
        const next = { ...cur, ...msg.status }
        const changed = RUNTIME_KEYS.some((k) => next[k] !== cur[k])
        if (changed) services.value.splice(i, 1, next)
      }
    } else if (msg.type === 'build') {
      buildRunning.value = msg.state === 'running'
    }
  })
  refresh()
  pollBuild()
  buildPoll = setInterval(pollBuild, 3000)
  try {
    const s = await rest.getSettings()
    if (s && s.scanDir) scanDir.value = s.scanDir
  } catch (e) { /* ignore */ }
})
onUnmounted(() => {
  off && off()
  clearInterval(buildPoll)
})
</script>

<style>
body { margin: 0; background: #f2f4f8; font-family: 'Segoe UI', system-ui, sans-serif; }
.app { min-height: 100vh; }
.topbar {
  display: flex; align-items: center; justify-content: space-between; gap: 16px;
  padding: 12px 24px; background: #fff;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  position: sticky; top: 0; z-index: 10; flex-wrap: wrap;
}
.brand { display: flex; align-items: center; gap: 10px; }
.brand-icon {
  width: 40px; height: 40px; border-radius: 10px; font-size: 20px;
  background: linear-gradient(135deg, #409eff, #7c3aed); color: #fff;
  display: flex; align-items: center; justify-content: center;
}
.brand-title { font-size: 16px; font-weight: 700; color: #1f2d3d; line-height: 1.2; }
.brand-sub { font-size: 11px; color: #909399; letter-spacing: .5px; }
.scan-box { display: flex; gap: 8px; align-items: center; flex: 1; min-width: 420px; justify-content: center; }
.actions { display: flex; align-items: center; gap: 8px; }
.ws-dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; margin-right: 4px; }
.ws-dot.on { background: #67c23a; box-shadow: 0 0 0 3px rgba(103,194,58,.2); }
.ws-dot.off { background: #f56c6c; box-shadow: 0 0 0 3px rgba(245,108,108,.2); }
.main { padding: 20px 24px; max-width: 1400px; margin: 0 auto; }
.group-title { display: inline-flex; gap: 10px; align-items: center; font-weight: 600; }
.count { color: #909399; font-weight: 400; font-size: 13px; }
.group-ops { margin-left: auto; margin-right: 16px; }
.build-bar { display: flex; gap: 8px; margin-bottom: 10px; }
.build-term { height: 420px; border: 1px solid #e8ecf2; border-radius: 8px; overflow: hidden; }
.groups {
  --el-collapse-border-color: transparent;
  background: #fff; border-radius: 12px; padding: 4px 16px;
  box-shadow: 0 1px 4px rgba(0,0,0,.05);
}
.groups .el-collapse-item__header { height: 52px; }
</style>
