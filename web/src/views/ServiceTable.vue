<template>
  <el-table :data="items" size="default" stripe>
    <el-table-column label="服务" min-width="230">
      <template #default="{ row }">
        <div class="svc-name">
          {{ row.name }}
          <el-tag v-if="row.type !== 'jar'" size="small" effect="plain" round>{{ typeLabel(row.type) }}</el-tag>
        </div>
        <div v-if="row.description" class="svc-desc" :title="row.description">{{ row.description }}</div>
        <div class="svc-path" :title="row.path">{{ row.path }}</div>
      </template>
    </el-table-column>
    <el-table-column label="状态" width="92">
      <template #default="{ row }">
        <span class="dot mm-dot" :class="[row.state, { beat: row.state === 'running' }]"></span>{{ stateText(row.state) }}
      </template>
    </el-table-column>
    <el-table-column prop="port" label="端口" width="64">
      <template #default="{ row }"><span class="num">{{ row.port || '-' }}</span></template>
    </el-table-column>
    <el-table-column label="健康" width="76">
      <template #default="{ row }">
        <el-tag :type="healthType(row)" size="small" effect="dark" round>{{ healthText(row) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="CPU" width="72">
      <template #default="{ row }"><span class="num">{{ fmt(row.cpu, '%') }}</span></template>
    </el-table-column>
    <el-table-column label="内存" width="82">
      <template #default="{ row }"><span class="num">{{ fmt(row.memMb, 'MB') }}</span></template>
    </el-table-column>
    <el-table-column label="线程" width="56">
      <template #default="{ row }"><span class="num">{{ row.state === 'stopped' ? '-' : (row.threads ?? '-') }}</span></template>
    </el-table-column>
    <el-table-column label="运行时长" width="84">
      <template #default="{ row }"><span class="num">{{ uptime(row) }}</span></template>
    </el-table-column>
    <el-table-column label="PID" width="64">
      <template #default="{ row }"><span class="num">{{ row.pid || '-' }}</span></template>
    </el-table-column>
    <el-table-column label="操作" width="310" fixed="right">
      <template #default="{ row }">
        <el-button link type="primary" :disabled="row.state !== 'stopped' && row.state !== 'failed'" @click="act('start', row.id)">启动</el-button>
        <el-button link type="danger" :disabled="row.state === 'stopped'" @click="act('stop', row.id)">停止</el-button>
        <el-button link type="primary" :disabled="row.state === 'stopped'" @click="act('restart', row.id)">重启</el-button>
        <el-divider direction="vertical" />
        <el-button link type="info" @click="$emit('open-log', row.id)">日志</el-button>
        <el-button link type="info" @click="$emit('open-metrics', row.id)">指标</el-button>
        <el-button link type="info" @click="openDir(row)">目录</el-button>
        <el-divider direction="vertical" />
        <el-button link type="warning" @click="openEdit(row)">编辑</el-button>
        <el-button link type="danger" @click="del(row)">删除</el-button>
      </template>
    </el-table-column>
  </el-table>

  <el-dialog v-model="editOpen" title="编辑服务" width="560px">
    <el-form label-width="90px">
      <el-form-item label="端口">
        <el-input-number v-model="form.port" :min="0" :max="65535" controls-position="right" />
        <span class="hint">用于状态探测与健康检查，0 = 不探测</span>
      </el-form-item>
      <el-form-item label="分组"><el-input v-model="form.group" placeholder="default" /></el-form-item>
      <el-form-item label="说明">
        <el-input v-model="form.description" type="textarea" :rows="2" placeholder="这个服务是干什么的" />
      </el-form-item>
      <el-form-item label="健康地址"><el-input v-model="form.healthUrl" placeholder="留空则按端口自动生成 /actuator/health" /></el-form-item>
      <template v-if="form.type === 'jar'">
        <el-form-item label="Java 参数"><el-input v-model="form.javaOpts" placeholder="-Xms96m -Xmx128m" /></el-form-item>
      </template>
      <template v-else>
        <el-form-item label="解释器">
          <el-select v-model="interp" placeholder="自动检测（可选）" clearable filterable style="width: 100%" @change="applyInterp">
            <el-option v-for="rt in runtimes" :key="rt.path" :label="rt.name" :value="rt.path" />
          </el-select>
        </el-form-item>
        <el-form-item label="启动命令"><el-input v-model="form.command" placeholder="python app.py" /></el-form-item>
        <el-form-item label="工作目录"><el-input v-model="form.workDir" /></el-form-item>
      </template>
      <el-form-item label="自动重启"><el-switch v-model="form.autoRestart" /></el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="editOpen = false">取消</el-button>
      <el-button type="primary" @click="saveEdit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { reactive, ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { rest } from '../api/ws.js'

defineProps({ items: { type: Array, default: () => [] } })
const emit = defineEmits(['open-log', 'open-metrics', 'refresh'])

const editOpen = ref(false)
const runtimes = ref([])
const interp = ref('')
const now = ref(Math.floor(Date.now() / 1000))
let nowTimer = null
const form = reactive({ id: '', port: 0, group: '', description: '', javaOpts: '', command: '', workDir: '', healthUrl: '', autoRestart: true, type: 'jar' })

onMounted(() => {
  nowTimer = setInterval(() => { now.value = Math.floor(Date.now() / 1000) }, 1000)
})
onUnmounted(() => clearInterval(nowTimer))

function loadRuntimes() {
  rest.runtimes().then((r) => { runtimes.value = r || [] })
}
watch(editOpen, (v) => { if (v) loadRuntimes() })

function applyInterp(path) {
  if (!path) return
  const cmd = (form.command || '').trim()
  let restCmd = cmd
  const m = cmd.match(/^("[^"]+"|\S+)\s*(.*)$/)
  if (m) {
    const first = m[1].replace(/^"|"$/g, '')
    if (/python(\.exe)?$/i.test(first) || /^(python|py)$/i.test(first)) {
      restCmd = m[2]
    }
  }
  const q = /\s/.test(path) ? `"${path}"` : path
  form.command = restCmd ? `${q} ${restCmd}` : q
}

async function act(action, id) {
  const res = await rest[action](id)
  if (res && res.error) {
    ElMessage({ message: res.error, type: 'error', duration: 6000, showClose: true })
    return
  }
  if (action === 'start') ElMessage.success('启动指令已发送')
  emit('refresh')
}
function openDir(row) {
  rest.openDir(row.id)
}
function openEdit(row) {
  Object.assign(form, {
    id: row.id, port: row.port || 0, group: row.group || '', description: row.description || '',
    javaOpts: row.javaOpts || '', command: row.command || '', workDir: row.workDir || '',
    healthUrl: row.healthUrl || '', autoRestart: !!row.autoRestart, type: row.type || 'jar'
  })
  editOpen.value = true
}
async function saveEdit() {
  const res = await rest.update(form.id, {
    port: form.port, group: form.group, description: form.description,
    javaOpts: form.javaOpts, command: form.command, workDir: form.workDir,
    healthUrl: form.healthUrl, autoRestart: form.autoRestart
  })
  if (res && res.error) {
    ElMessage.error(res.error)
    return
  }
  ElMessage.success('已保存')
  editOpen.value = false
  emit('refresh')
}
async function del(row) {
  try {
    await ElMessageBox.confirm(`确定从面板移除「${row.name}」？（不会删除文件）`, '删除服务', { type: 'warning' })
  } catch (e) {
    return
  }
  const res = await rest.remove(row.id)
  if (res && res.error) {
    ElMessage.error(res.error)
    return
  }
  ElMessage.success('已移除')
  emit('refresh')
}
function typeLabel(t) {
  return { python: 'Python', node: 'Node', go: 'Go', custom: '自定义' }[t] || t
}
function stateText(s) {
  return { running: '运行中', starting: '启动中', stopped: '已停止', failed: '异常退出' }[s] || s
}
function healthText(row) {
  if (row.state === 'stopped') return '—'
  return row.health || 'unknown'
}
function healthType(row) {
  if (row.state === 'stopped') return 'info'
  return { UP: 'success', DOWN: 'danger' }[row.health] || 'warning'
}
function fmt(v, unit) {
  if (v == null || v === 0) return '0 ' + unit
  return (Math.round(v * 10) / 10) + ' ' + unit
}
function uptime(row) {
  if (!row.uptime || row.state === 'stopped') return '-'
  const sec = Math.max(0, now.value - row.uptime)
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  return h > 0 ? `${h}h${m}m` : m > 0 ? `${m}m${s}s` : `${s}s`
}
</script>

<style scoped>
.svc-name { font-weight: 700; display: flex; align-items: center; gap: 6px; }
.svc-desc { font-size: 12px; color: #4f46e5; margin-top: 1px; max-width: 320px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.svc-path { font-family: var(--mm-mono); font-size: 10.5px; color: #9a96b3; margin-top: 2px; max-width: 320px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.num { font-family: var(--mm-mono); font-size: 12.5px; }
.dot { display: inline-block; width: 8px; height: 8px; margin-right: 6px; }
.dot.running { background: #22c55e; animation: mm-pulse 2s infinite; }
.dot.starting { background: #f59e0b; animation: mm-pulse-amber 1.2s infinite; }
.dot.stopped { background: #c0c4cc; }
.dot.failed { background: #ef4444; }
@keyframes mm-pulse-amber {
  0% { box-shadow: 0 0 0 0 rgba(245,158,11,.45); }
  70% { box-shadow: 0 0 0 5px rgba(245,158,11,0); }
  100% { box-shadow: 0 0 0 0 rgba(245,158,11,0); }
}
:deep(.el-table) { --el-table-header-bg-color: #faf9f6; --el-table-header-text-color: #8a86a8; --el-table-row-hover-bg-color: #f4f2fc; }
:deep(.el-table th) { font-family: var(--mm-mono); font-size: 11px; letter-spacing: .6px; text-transform: uppercase; }
.hint { margin-left: 10px; font-size: 12px; color: var(--el-text-color-secondary); }
</style>
