<template>
  <div class="wrap">
    <div class="now-bar">
      <div class="metric-chip">
        <span class="k">CPU</span>
        <span class="v">{{ lastCpu }}<small>%</small></span>
      </div>
      <div class="metric-chip">
        <span class="k">内存</span>
        <span class="v">{{ lastMem }}<small>MB</small></span>
      </div>
      <div class="metric-chip">
        <span class="k">线程</span>
        <span class="v">{{ lastThreads || '-' }}</span>
      </div>
    </div>
    <div ref="el" class="chart"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import { onMessage } from '../api/ws.js'

const props = defineProps({ serviceId: { type: String, required: true } })
const el = ref(null)
let chart = null
let off = null
const MAX = 120
let data = { cpu: [], mem: [], t: [] }
const lastCpu = ref('0.0')
const lastMem = ref('0')
const lastThreads = ref(0)

function reset() {
  data = { cpu: [], mem: [], t: [] }
  render()
}

function push(st) {
  const now = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  data.t.push(now)
  data.cpu.push(Math.round((st.cpu || 0) * 10) / 10)
  data.mem.push(Math.round((st.memMb || 0) * 10) / 10)
  lastCpu.value = (Math.round((st.cpu || 0) * 10) / 10).toFixed(1)
  lastMem.value = String(Math.round(st.memMb || 0))
  if (st.threads) lastThreads.value = st.threads
  if (data.t.length > MAX) {
    data.t.shift(); data.cpu.shift(); data.mem.shift()
  }
  render()
}

function render() {
  if (!chart) return
  const grad = (c1, c2) => new echarts.graphic.LinearGradient(0, 0, 0, 1, [
    { offset: 0, color: c1 }, { offset: 1, color: c2 }
  ])
  chart.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1e2235',
      borderColor: '#1e2235',
      textStyle: { color: '#e8eaf2', fontSize: 12 },
      valueFormatter: (v) => v
    },
    legend: {
      bottom: 4, icon: 'round', itemWidth: 14, itemHeight: 4,
      textStyle: { color: '#6b7280', fontSize: 12 }
    },
    grid: { left: 56, right: 60, top: 18, bottom: 56 },
    xAxis: {
      type: 'category', boundaryGap: false, data: data.t,
      axisLine: { lineStyle: { color: '#d9d5cc' } },
      axisTick: { show: false },
      axisLabel: { color: '#8a86a8', fontFamily: 'Consolas', fontSize: 10 }
    },
    yAxis: [
      {
        type: 'value', name: 'CPU %', min: 0, max: 100, splitNumber: 4,
        nameTextStyle: { color: '#8a86a8', fontFamily: 'Consolas' },
        axisLabel: { color: '#8a86a8', fontFamily: 'Consolas', fontSize: 10 },
        splitLine: { lineStyle: { color: '#e9e6de', type: 'dashed' } }
      },
      {
        type: 'value', name: 'MB', min: 0, splitNumber: 4,
        nameTextStyle: { color: '#8a86a8', fontFamily: 'Consolas' },
        axisLabel: { color: '#8a86a8', fontFamily: 'Consolas', fontSize: 10 },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: 'CPU %', type: 'line', smooth: 0.4, showSymbol: false, data: data.cpu,
        lineStyle: { width: 2, color: '#4f46e5' },
        itemStyle: { color: '#4f46e5' },
        areaStyle: { color: grad('rgba(79,70,229,.30)', 'rgba(79,70,229,.01)') }
      },
      {
        name: '内存 MB', type: 'line', smooth: 0.4, showSymbol: false, yAxisIndex: 1, data: data.mem,
        lineStyle: { width: 2, color: '#10b981' },
        itemStyle: { color: '#10b981' },
        areaStyle: { color: grad('rgba(16,185,129,.22)', 'rgba(16,185,129,.01)') }
      }
    ]
  })
}

onMounted(async () => {
  await nextTick()
  await new Promise((r) => setTimeout(r, 260))
  if (!el.value) return
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
.wrap { padding: 4px 6px 8px; }
.now-bar { display: flex; gap: 12px; margin-bottom: 8px; }
.metric-chip {
  flex: 1; background: #faf9f6; border: 1px solid #e8e5dd; border-radius: 10px;
  padding: 10px 16px; display: flex; align-items: baseline; gap: 10px;
}
.metric-chip .k { font-family: var(--mm-mono); font-size: 11px; color: #8a86a8; letter-spacing: 1px; }
.metric-chip .v { font-family: var(--mm-mono); font-size: 22px; font-weight: 700; color: #1e2235; }
.metric-chip .v small { font-size: 11px; color: #8a86a8; margin-left: 3px; }
.chart { width: 100%; height: 360px; }
</style>
