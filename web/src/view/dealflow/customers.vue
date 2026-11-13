<template>
  <section aria-label="客户列表">
    <h2 class="mb-4 text-lg font-semibold">客户</h2>
    <form class="mb-4 flex flex-wrap items-center gap-3" @submit.prevent="search.page = 1; load()">
      <label for="customer-keyword">客户名称</label>
      <el-input id="customer-keyword" v-model="search.keyword" class="max-w-xs" maxlength="128" />
      <label for="customer-sort">排序</label>
      <select id="customer-sort" v-model="search.sort" class="rounded border p-2">
        <option value="id">创建顺序</option><option value="name">名称</option><option value="code">编号</option><option value="nextFollowupAt">下次跟进</option>
      </select>
      <label for="customer-followup">跟进状态</label>
      <select id="customer-followup" v-model="search.followup" class="rounded border p-2"><option value="">全部</option><option value="overdue">已逾期</option><option value="scheduled">待跟进</option><option value="none">未安排</option></select>
      <label for="customer-order">方向</label>
      <select id="customer-order" v-model="search.order" class="rounded border p-2">
        <option value="desc">降序</option><option value="asc">升序</option>
      </select>
      <el-button native-type="submit" :disabled="saving">查询</el-button>
    </form>
    <p v-if="error" role="alert">{{ error }}</p>
    <el-table :data="result.list" row-key="id" v-loading="loading" empty-text="当前筛选下暂无可见客户">
      <el-table-column label="客户名称" min-width="160">
        <template #default="{ row }"><RouterLink :to="{ name: 'DealflowCustomer', params: { id: row.id } }">{{ row.name }}</RouterLink></template>
      </el-table-column>
      <el-table-column label="业务编号" prop="code" min-width="200" />
      <el-table-column label="负责人" prop="ownerName" />
      <el-table-column label="部门" prop="departmentName" />
      <el-table-column label="行业" prop="industry" />
      <el-table-column label="下次跟进" min-width="180"><template #default="{ row }">{{ row.nextFollowupAt ? new Date(row.nextFollowupAt).toLocaleString() : '未安排' }}</template></el-table-column>
    </el-table>
    <el-pagination class="my-4" layout="total, sizes, prev, pager, next" :total="result.total"
      :page-sizes="[10, 20, 50, 100]" :current-page="search.page" :page-size="search.pageSize"
      @current-change="changePage" @size-change="changeSize" />
    <form v-if="canCreate" aria-label="新增客户" class="mt-6 grid max-w-xl gap-3" @submit.prevent="create">
      <h3 class="font-semibold">新增客户</h3>
      <label for="new-customer-name">客户名称</label><el-input id="new-customer-name" v-model="draft.name" required maxlength="128" />
      <label for="new-customer-industry">行业</label><el-input id="new-customer-industry" v-model="draft.industry" maxlength="64" />
      <label for="new-customer-remark">备注</label><el-input id="new-customer-remark" v-model="draft.remark" type="textarea" maxlength="512" />
      <el-button type="primary" native-type="submit" :loading="saving">创建客户</el-button>
    </form>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createCustomer, listCustomers, type CustomerPage } from '@/api/dealflow-crm'
import { useDealflowSession } from '@/pinia/modules/dealflow-session'
const props = defineProps<{ tenantId: string; canCreate: boolean }>()
const router = useRouter()
const session = useDealflowSession()
const search = reactive({ page: 1, pageSize: 20, sort: 'id', order: 'desc', keyword: '', followup: '' })
const result = ref<CustomerPage>({ list: [], total: 0, page: 1, pageSize: 20 })
const draft = reactive({ name: '', industry: '', remark: '' })
const loading = ref(false), saving = ref(false), error = ref('')
const lifetime = new AbortController()
let reading: AbortController | undefined
function abort() { lifetime.abort(); reading?.abort() }
watch(() => session.menu, abort, { flush: 'sync' })
onBeforeUnmount(abort)
onMounted(load)
async function load() {
  if (lifetime.signal.aborted) return
  reading?.abort()
  const request = new AbortController()
  reading = request
  loading.value = true
  error.value = ''
  result.value = { list: [], total: 0, page: search.page, pageSize: search.pageSize }
  try {
    const page = await listCustomers(props.tenantId, { ...search }, request.signal)
    if (!request.signal.aborted) result.value = page
  } catch {
    if (!request.signal.aborted) error.value = '无法加载客户，请检查当前企业和权限。'
  } finally { if (!request.signal.aborted) loading.value = false }
}
function changePage(page: number) { search.page = page; void load() }
function changeSize(size: number) { search.pageSize = size; search.page = 1; void load() }
async function create() {
  if (saving.value || lifetime.signal.aborted || !props.canCreate) return
  saving.value = true
  error.value = ''
  try {
    const row = await createCustomer(props.tenantId, { ...draft }, lifetime.signal)
    if (!lifetime.signal.aborted) await router.push({ name: 'DealflowCustomer', params: { id: row.id } })
  } catch { if (!lifetime.signal.aborted) error.value = '无法创建客户，请检查表单和当前权限。' }
  finally { if (!lifetime.signal.aborted) saving.value = false }
}
</script>
