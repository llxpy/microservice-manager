<template>
  <div>
    <div v-for="group in groups" :key="group.name" class="group">
      <div class="group-head">
        <span class="group-name" @click="group.open = !group.open">
          {{ group.open ? '▾' : '▸' }} 分组: {{ group.name }}
          <span class="count">({{ group.items.length }})</span>
        </span>
        <div>
          <button class="btn small" @click="$emit('refresh'); doGroup(group.name, 'start')">▶ 启动整组</button>
          <button class="btn small danger" @click="doGroup(group.name, 'stop')">■ 停止整组</button>
        </div>
      </div>
      <table v-show="group.open" class="svc-table">
        <thead>
          <tr>
            <th>名称</th><th>状态</th><th>端口</th><th>健康</th>
            <th>CPU</th><th>内存</th><th>线程</th><th>运行时长</th><th>PID</th><th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in group.items" :key="s.id">
            <td>
              <div class="svc-name">{{ s.name }}</div>
              <div class="svc-path" :title="s.path">{{ s.path }}</div>
            </td>
            <td><span class="dot" :class="s.state"></span> {{ stateText(s.state) }}</td>
            <td>{{ s.port || '-' }}</td>
            <td><span class="h-badge" :class="healthClass(s.health, s.state)">{{ healthText(s) }}</span></td>
            <td>{{ fmt(s.cpu, '%') }}</td>
            <td>{{ fmt(s.memMb, 'MB') }}</td>
            <td>{{ s.state === 'stopped' ? '-' : (s.threads ?? '-') }}</td>
            <td>{{ uptime(s) }}</td>
            <td>{{ s.pid || '-' }}</td>
            <td class="ops">
              <button class="btn small primary" :disabled="s.state !== 'stopped' && s.state !== 'failed'" @click="act('start', s.id)">启动</button>
              <button class="btn small danger" :disabled="s.state === 'stopped'" @click="act('stop', s.id)">停止</button>
              <button class="btn small" :disabled="s.state === 'stopped'" @click="act('restart', s.id)">重启</button>
              <button class="btn small" @click="$emit('open-log', s.id)">日志</button>
              <button class="btn small" @click="$emit('open-metrics', s.id)">指标</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
import { computed, reactive, watch } from 'vue'
import { rest } from '../api/ws.js'

const props = defineProps({ services: { type: Array, default: () => [] } })
defineEmits(['open-log', 'open-metrics', 'refresh'])

const openMap = reactive({})
const groups = computed(() => {
  const map = {}
  for (const s of props.services) {
    const g = s.group || 'default'
    if (!map[g]) map[g] = []
    map[g].push(s)
  }
  return Object.keys(map).sort().map((name) => ({
    name,
    open: openMap[name] !== false,
    items: map[name]
  }))
})

watch(groups, (gs) => gs.forEach((g) => { if (!(g.name in openMap)) openMap[g.name] = true }), { immediate: true })

function act(action, id) {
  rest[action](id)
}
function doGroup(name, action) {
  action === 'start' ? rest.groupStart(name) : rest.groupStop(name)
}
function stateText(s) {
  return { running: '运行中', starting: '启动中', stopped: '已停止', failed: '异常退出' }[s] || s
}
function healthText(s) {
  if (s.state === 'stopped') return '—'
  return s.health || 'unknown'
}
function healthClass(h, state) {
  if (state === 'stopped') return 'na'
  return { UP: 'up', DOWN: 'down' }[h] || 'unknown'
}
function fmt(v, unit) {
  if (v == null || v === 0) return '0 ' + unit
  return (Math.round(v * 10) / 10) + ' ' + unit
}
function uptime(s) {
  if (!s.uptime || s.state === 'stopped') return '-'
  const sec = Math.floor(Date.now() / 1000 - s.uptime)
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60)
  return h > 0 ? `${h}h${m}m` : `${m}m${sec % 60}s`
}
</script>

<style scoped>
.group { margin-bottom: 18px; background: #151b27; border: 1px solid #232c3d; border-radius: 10px; overflow: hidden; }
.group-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 9px 14px; background: #1a2233;
}
.group-name { cursor: pointer; font-weight: 600; }
.count { color: #7d8aa5; font-weight: 400; font-size: 12px; }
.svc-table { width: 100%; border-collapse: collapse; }
.svc-table th, .svc-table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid #1e2739; }
.svc-table th { color: #7d8aa5; font-size: 12px; font-weight: 500; }
.svc-name { font-weight: 600; }
.svc-path { font-size: 11px; color: #5b6a85; max-width: 260px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 5px; }
.dot.running { background: #22c55e; }
.dot.starting { background: #eab308; }
.dot.stopped { background: #6b7280; }
.dot.failed { background: #ef4444; }
.h-badge { padding: 2px 8px; border-radius: 10px; font-size: 12px; }
.h-badge.up { background: #14321f; color: #4ade80; }
.h-badge.down { background: #3a1414; color: #f87171; }
.h-badge.unknown { background: #2b2b1a; color: #facc15; }
.h-badge.na { color: #5b6a85; }
.ops { white-space: nowrap; }
.ops .btn { margin-right: 4px; }
</style>
