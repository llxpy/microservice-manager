<template>
  <el-table :data="items" size="default" stripe>
    <el-table-column label="名称" min-width="200">
      <template #default="{ row }">
        <div class="svc-name">{{ row.name }}</div>
        <div class="svc-path" :title="row.path">{{ row.path }}</div>
      </template>
    </el-table-column>
    <el-table-column label="状态" width="100">
      <template #default="{ row }">
        <span class="dot" :class="row.state"></span>{{ stateText(row.state) }}
      </template>
    </el-table-column>
    <el-table-column prop="port" label="端口" width="70">
      <template #default="{ row }">{{ row.port || '-' }}</template>
    </el-table-column>
    <el-table-column label="健康" width="90">
      <template #default="{ row }">
        <el-tag :type="healthType(row)" size="small" effect="dark">{{ healthText(row) }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="CPU" width="80">
      <template #default="{ row }">{{ fmt(row.cpu, '%') }}</template>
    </el-table-column>
    <el-table-column label="内存" width="90">
      <template #default="{ row }">{{ fmt(row.memMb, 'MB') }}</template>
    </el-table-column>
    <el-table-column label="线程" width="60">
      <template #default="{ row }">{{ row.state === 'stopped' ? '-' : (row.threads ?? '-') }}</template>
    </el-table-column>
    <el-table-column label="运行时长" width="90">
      <template #default="{ row }">{{ uptime(row) }}</template>
    </el-table-column>
    <el-table-column label="PID" width="70">
      <template #default="{ row }">{{ row.pid || '-' }}</template>
    </el-table-column>
    <el-table-column label="操作" width="300" fixed="right">
      <template #default="{ row }">
        <el-button size="small" type="primary" :disabled="row.state !== 'stopped' && row.state !== 'failed'" @click="act('start', row.id)">启动</el-button>
        <el-button size="small" type="danger" :disabled="row.state === 'stopped'" @click="act('stop', row.id)">停止</el-button>
        <el-button size="small" :disabled="row.state === 'stopped'" @click="act('restart', row.id)">重启</el-button>
        <el-button size="small" @click="$emit('open-log', row.id)">日志</el-button>
        <el-button size="small" @click="$emit('open-metrics', row.id)">指标</el-button>
      </template>
    </el-table-column>
  </el-table>
</template>

<script setup>
import { rest } from '../api/ws.js'

defineProps({ items: { type: Array, default: () => [] } })
defineEmits(['open-log', 'open-metrics', 'refresh'])

function act(action, id) {
  rest[action](id)
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
  const sec = Math.floor(Date.now() / 1000 - row.uptime)
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60)
  return h > 0 ? `${h}h${m}m` : `${m}m${sec % 60}s`
}
</script>

<style scoped>
.svc-name { font-weight: 600; }
.svc-path { font-size: 11px; color: var(--el-text-color-secondary); max-width: 320px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; }
.dot.running { background: var(--el-color-success); }
.dot.starting { background: var(--el-color-warning); }
.dot.stopped { background: var(--el-color-info); }
.dot.failed { background: var(--el-color-danger); }
</style>
