import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { getMenu, getWorkspaces, type MenuItem, type Workspace } from '@/api/dealflow'

const selectionKey = 'dealflow-workspace-id'

export const useDealflowSession = defineStore('dealflow-session', () => {
  const workspaces = ref<Workspace[]>([])
  const tenantId = ref('')
  const workspace = computed(() => workspaces.value.find(item => item.id === tenantId.value))
  const menu = ref<MenuItem[] | null>(null)
  const loading = ref(false)
  const error = ref('')
  let accountRequest: AbortController | undefined
  let menuRequest: AbortController | undefined

  async function select(id: string) {
    if (!workspaces.value.some(item => item.id === id)) return
    menuRequest?.abort()
    const request = new AbortController()
    menuRequest = request
    menu.value = null
    tenantId.value = id
    sessionStorage.setItem(selectionKey, id)
    error.value = ''
    loading.value = true
    try {
      const items = await getMenu(id, request.signal)
      if (!request.signal.aborted) menu.value = items
    } catch {
      if (!request.signal.aborted) error.value = '无法打开当前企业，请重新选择或重试。'
    } finally {
      if (!request.signal.aborted) loading.value = false
    }
  }

  function reset() {
    accountRequest?.abort()
    menuRequest?.abort()
    menu.value = null
    tenantId.value = ''
    workspaces.value = []
    error.value = ''
    loading.value = false
  }

  async function initialize() {
    reset()
    const request = new AbortController()
    accountRequest = request
    loading.value = true
    try {
      const items = await getWorkspaces(request.signal)
      if (request.signal.aborted) return
      workspaces.value = items
      const saved = sessionStorage.getItem(selectionKey)
      const selected = items.find(item => item.id === saved) ?? items[0]
      if (selected) {
        void select(selected.id)
      } else {
        sessionStorage.removeItem(selectionKey)
        loading.value = false
      }
    } catch {
      if (!request.signal.aborted) {
        error.value = '无法加载可选企业，请重试。'
        loading.value = false
      }
    }
  }

  return { workspaces, workspace, menu, loading, error, select, initialize, reset }
})
