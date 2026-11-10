<template>
  <section aria-label="部门">
    <p v-if="loading" role="status">正在加载部门…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="department">
      <h2 class="mb-2 text-lg font-semibold">{{ department.name }}</h2>
      <p class="mb-3">部门编码：{{ department.code }}</p>
      <form v-if="renaming" class="flex flex-wrap items-center gap-3" @submit.prevent="save">
        <label for="dealflow-department-name">部门名称</label>
        <el-input id="dealflow-department-name" v-model="name" class="max-w-xs" required />
        <el-button native-type="submit" type="primary" :loading="saving">保存</el-button>
        <RouterLink :to="detailRoute">取消</RouterLink>
      </form>
      <RouterLink
        v-else-if="canRename"
        :to="{ name: 'DealflowRenameDepartment', params: { id: departmentId } }"
      >
        修改部门名称
      </RouterLink>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getDepartment, renameDepartment, type Department } from '@/api/dealflow'
import { useDealflowSession } from '@/pinia/modules/dealflow-session'

const props = defineProps<{
  tenantId: string
  departmentId: string
  canRename: boolean
  renaming: boolean
}>()
const session = useDealflowSession()
const router = useRouter()
const request = new AbortController()
const department = ref<Department>()
const name = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const detailRoute = { name: 'DealflowDepartment', params: { id: props.departmentId } }

// A switch invalidates this page synchronously, before Vue removes the old view.
watch(() => session.menu, () => request.abort(), { flush: 'sync' })
onBeforeUnmount(() => request.abort())

onMounted(async () => {
  try {
    const result = await getDepartment(props.tenantId, props.departmentId, request.signal)
    if (request.signal.aborted) return
    department.value = result
    name.value = result.name
  } catch {
    if (!request.signal.aborted) error.value = '部门不存在、无权访问或暂时无法加载。'
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
})

async function save() {
  if (saving.value || request.signal.aborted || !props.canRename) return
  error.value = ''
  saving.value = true
  try {
    await renameDepartment(props.tenantId, props.departmentId, name.value, request.signal)
    if (!request.signal.aborted) await router.replace(detailRoute)
  } catch {
    if (!request.signal.aborted) error.value = '无法修改部门名称，请检查名称和当前权限。'
  } finally {
    if (!request.signal.aborted) saving.value = false
  }
}
</script>
