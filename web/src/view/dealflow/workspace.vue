<template>
  <main class="mx-auto h-full max-w-4xl overflow-auto p-6">
    <header class="mb-5 flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">DealFlow 工作台</h1>
      <RouterLink to="/">返回系统首页</RouterLink>
    </header>

    <el-card>
      <div v-if="session.workspaces.length" class="mb-4 flex flex-wrap items-center gap-3">
        <label for="dealflow-workspace">当前企业</label>
        <select
          id="dealflow-workspace"
          class="rounded border border-solid border-gray-300 bg-transparent p-2"
          :value="session.workspace?.id"
          :disabled="switching"
          @change="switchWorkspace"
        >
          <option v-for="item in session.workspaces" :key="item.id" :value="item.id">
            {{ item.name }}
          </option>
        </select>
        <span v-if="session.workspace && session.menu">
          当前角色：{{ roleNames[session.workspace.roleKey] ?? session.workspace.roleKey }}
        </span>
      </div>

      <p v-if="session.loading || switching" role="status">正在加载工作台…</p>
      <div v-else-if="session.error" role="alert">
        {{ session.error }}
        <el-button @click="session.initialize()">重试</el-button>
      </div>
      <p v-else-if="!session.workspace" role="alert">当前账号没有可访问的企业。</p>
      <template v-else>
        <nav v-if="departments" aria-label="企业菜单" class="mb-4 flex gap-4">
          <RouterLink :to="{ name: 'DealflowWorkspace' }">工作台</RouterLink>
          <RouterLink :to="{ name: 'DealflowDepartment', params: { id: session.workspace.departmentId } }">
            我的部门
          </RouterLink>
          <RouterLink v-if="customers" :to="{ name: 'DealflowCustomers' }">客户</RouterLink>
        </nav>
        <p v-if="!departments || (route.meta.renameDepartment && !departments.canRename) || (route.meta.customers && !customers)" role="alert">
          无权访问此页面。
        </p>
        <p v-else-if="route.name === 'DealflowWorkspace'">从企业菜单选择功能。</p>
        <RouterView
          v-else
          :key="`${session.workspace.id}:${route.fullPath}`"
          :tenant-id="session.workspace.id"
          :department-id="String(route.params.id)"
          :can-rename="departments.canRename"
          :renaming="Boolean(route.meta.renameDepartment)"
          :can-create="Boolean(customers?.canCreate)"
        />
      </template>
    </el-card>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDealflowSession } from '@/pinia/modules/dealflow-session'
import { useUserStore } from '@/pinia/modules/user'

const roleNames: Record<string, string> = {
  tenant_admin: '企业管理员',
  sales: '销售',
  manager: '经理',
  finance: '财务'
}

const session = useDealflowSession()
const user = useUserStore()
const router = useRouter()
const route = useRoute()
const switching = ref(false)
const departments = computed(() => session.menu?.find(item => item.key === 'departments'))
const customers = computed(() => session.menu?.find(item => item.key === 'customers'))

async function switchWorkspace(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  if (switching.value || id === session.workspace?.id) return
  switching.value = true
  void session.select(id)
  try {
    await router.replace({ name: 'DealflowWorkspace' })
  } finally {
    switching.value = false
  }
}

watch(() => Boolean(user.token), (loggedIn) => {
  if (loggedIn) return
  session.reset()
  void router.replace({ name: 'Login' })
}, { flush: 'sync' })
onMounted(session.initialize)
onBeforeUnmount(session.reset)
</script>
