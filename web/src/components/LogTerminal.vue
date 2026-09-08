<template>
  <div class="term-wrap">
    <div ref="el" class="terminal"></div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { onMessage, rest } from '../api/ws.js'
import 'xterm/css/xterm.css'

const props = defineProps({ serviceId: { type: String, required: true } })
const el = ref(null)
let term = null
let fit = null
let off = null

function clear() {
  term.clear()
}

function loadHistory() {
  rest.logs(props.serviceId, 300).then((res) => {
    if (res && res.lines) {
      term.writeln('\x1b[2m--- 历史日志 ---\x1b[0m')
      res.lines.forEach((l) => term.write(l.replace(/\n$/, '\r\n')))
      term.writeln('\x1b[2m--- 实时日志 ---\x1b[0m')
      fit.fit()
    }
  })
}

onMounted(() => {
  term = new Terminal({
    convertEol: true,
    fontSize: 12,
    theme: { background: '#0d1117', foreground: '#c9d1d9' },
    scrollback: 5000,
    disableStdin: true
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(el.value)
  fit.fit()

  off = onMessage((msg) => {
    if (msg.type === 'log' && msg.serviceId === props.serviceId) {
      term.write(msg.data.replace(/\n$/, '\r\n'))
    }
  })
  loadHistory()
})

watch(() => props.serviceId, () => {
  if (!term) return
  term.clear()
  loadHistory()
})

onUnmounted(() => {
  off && off()
  term && term.dispose()
})

defineExpose({ clear })
</script>

<style scoped>
.term-wrap { height: 100%; min-height: 0; background: #0d1117; border-radius: 6px; overflow: hidden; padding: 8px; }
.terminal { width: 100%; height: 100%; }
</style>
