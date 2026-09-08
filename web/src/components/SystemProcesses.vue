<template>
  <div>
    <div class="bar">
      <el-input v-model="keyword" placeholder="按名称 / PID / 路径筛选" clearable style="width: 280px" />
      <el-button :loading="loading" @click="load">刷新</el-button>
      <span class="count">共 {{ filtered.length }} 个进程</span>
    </div>
    <el-table :data="filtered" height="480" size="small" row-key="pid" v-loading="loading && !rows.length">
      <el-table-column prop="pid" label="PID" width="80" />
      <el-table-column label="名称" width="200">
        <template #default="{ row }">
          {{ row.name }}
          <el-tag v-if="row.self" size="small" effect="plain" round>本面板</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="CPU" width="80">
        <template #default="{ row }">{{ (Math.round(row.cpu * 10) / 10) + '%' }}</template>
      </el-table-column>
      <el-table-column label="内存" width="90" sortable :sort-method="(a, b) => a.memMb - b.memMb">
        <template #default="{ row }">{{ (Math.round(row.memMb * 10) / 10) + ' MB' }}</template>
      </el-table-column>
      <el-table-column label="路径" min-width="260">
        <template #default="{ row }">
          <span class="exe" :title="row.exe">{{ row.exe || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <el-button link type="info" :disabled="!row.exe" @click="locate(row)">定位</el-button>
          <el-button link type="danger" :disabled="row.self" @click="kill(row)">结束</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { rest } from '../api/ws.js'

const rows = ref([])
const keyword = ref('')
const loading = ref(false)
let timer = null

const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return rows.value
  return rows.value.filter((r) =>
    r.name.toLowerCase().includes(k) || String(r.pid).includes(k) || (r.exe || '').toLowerCase().includes(k)
  )
})

async function load() {
  loading.value = true
  try {
    const list = await rest.sysProcesses()
    if (list) rows.value = list
  } finally {
    loading.value = false
  }
}

async function kill(row) {
  try {
    await ElMessageBox.confirm(`确定结束进程「${row.name}」(PID ${row.pid}) 及其子进程？`, '结束进程', { type: 'warning' })
  } catch (e) {
    return
  }
  const res = await rest.sysKill(row.pid)
  if (res && res.error) {
    ElMessage.error(res.error)
    return
  }
  ElMessage.success('已结束')
  load()
}

function locate(row) {
  rest.sysOpenDir(row.exe)
}

onMounted(() => {
  load()
  timer = setInterval(load, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.bar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; }
.count { margin-left: auto; color: var(--el-text-color-secondary); font-size: 13px; }
.exe { font-size: 12px; color: var(--el-text-color-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block; }
</style>
