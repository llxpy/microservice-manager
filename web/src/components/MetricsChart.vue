<template>
  <div ref="el" class="chart"></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import { onMessage } from '../api/ws.js'

const props = defineProps({ serviceId: { type: String, required: true } })
const el = ref(null)
let chart = null
let off = null
const MAX = 60
let data = { cpu: [], mem: [], t: [] }

function reset() {
  data = { cpu: [], mem: [], t: [] }
  render()
}

function push(st) {
  const now = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  data.t.push(now)
  data.cpu.push(Math.round((st.cpu || 0) * 10) / 10)
  data.mem.push(Math.round((st.memMb || 0) * 10) / 10)
  if (data.t.length > MAX) {
    data.t.shift(); data.cpu.shift(); data.mem.shift()
  }
  render()
}

function render() {
  if (!chart) return
  chart.setOption({
    animation: false,
    tooltip: { trigger: 'axis' },
    legend: { data: ['CPU %', '内存 MB'], textStyle: { color: '#606266' } },
    grid: { left: 50, right: 20, top: 36, bottom: 28 },
    xAxis: { type: 'category', data: data.t, axisLabel: { color: '#606266' } },
    yAxis: [
      { type: 'value', name: 'CPU %', axisLabel: { color: '#606266' }, splitLine: { lineStyle: { color: '#e8ecf2' } } },
      { type: 'value', name: 'MB', axisLabel: { color: '#606266' }, splitLine: { show: false } }
    ],
    series: [
      { name: 'CPU %', type: 'line', data: data.cpu, showSymbol: false, lineStyle: { color: '#3b82f6' }, itemStyle: { color: '#3b82f6' }, areaStyle: { color: 'rgba(59,130,246,.15)' } },
      { name: '内存 MB', type: 'line', yAxisIndex: 1, data: data.mem, showSymbol: false, lineStyle: { color: '#22c55e' }, itemStyle: { color: '#22c55e' }, areaStyle: { color: 'rgba(34,197,94,.12)' } }
    ]
  })
}

onMounted(() => {
  chart = echarts.init(el.value)
  render()
  off = onMessage((msg) => {
    if (msg.type === 'status' && msg.serviceId === props.serviceId && msg.status) {
      push(msg.status)
    }
  })
  window.addEventListener('resize', resize)
})

function resize() { chart && chart.resize() }

watch(() => props.serviceId, reset)

onUnmounted(() => {
  off && off()
  window.removeEventListener('resize', resize)
  chart && chart.dispose()
})
</script>

<style scoped>
.chart { width: 100%; height: 380px; padding: 8px; }
</style>
