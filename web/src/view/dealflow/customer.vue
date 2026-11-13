<template>
  <section aria-label="客户详情">
    <RouterLink :to="{ name: 'DealflowCustomers' }">返回客户列表</RouterLink>
    <p v-if="loading" role="status">正在加载客户…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="customer">
      <h2 class="my-3 text-lg font-semibold">{{ customer.name }}</h2>
      <p>业务编号 {{ customer.code }}</p>
      <p class="mb-4">负责人 {{ customer.ownerName }} · 部门 {{ customer.departmentName }}</p>
      <form v-if="customer.canUpdate" aria-label="编辑客户" class="grid max-w-xl gap-2" @submit.prevent="saveCustomer">
        <label for="edit-customer-name">客户名称</label><el-input id="edit-customer-name" v-model="draft.name" required maxlength="128" />
        <label for="edit-customer-industry">行业</label><el-input id="edit-customer-industry" v-model="draft.industry" maxlength="64" />
        <label for="edit-customer-remark">备注</label><el-input id="edit-customer-remark" v-model="draft.remark" type="textarea" maxlength="512" />
        <el-button type="primary" native-type="submit" :loading="saving">保存客户</el-button>
      </form>
      <template v-else><p>行业 {{ customer.industry || '未填写' }}</p><p>备注 {{ customer.remark || '未填写' }}</p></template>
      <p class="mt-3">下次跟进：{{ customer.nextFollowupAt ? new Date(customer.nextFollowupAt).toLocaleString() : '未安排' }}</p>
      <h3 class="mb-3 mt-6 font-semibold">跟进时间线</h3>
      <p v-if="!history.total">暂无跟进记录。</p>
      <ol><li v-for="item in history.list" :key="item.id" class="my-3 whitespace-pre-wrap"><p>{{ item.actorName }} · {{ new Date(item.createdAt).toLocaleString() }}</p><p>{{ item.content }}</p></li></ol>
      <el-pagination v-if="history.total" layout="total, prev, pager, next" :total="history.total" :page-size="20" :current-page="history.page" @current-change="historyPage" />
      <form v-if="customer.canUpdate" aria-label="新增跟进" class="mt-4 grid max-w-xl gap-2" @submit.prevent="saveFollowup">
        <label for="followup-content">跟进内容</label><el-input id="followup-content" v-model="followup.content" type="textarea" required maxlength="1024" />
        <label for="followup-next">下次跟进时间（留空取消安排）</label><input id="followup-next" v-model="followup.next" type="datetime-local" class="rounded border p-2" />
        <el-button native-type="submit" type="primary" :loading="saving">保存跟进</el-button>
      </form>
      <form v-if="customer.canTransfer" aria-label="转移负责人" class="mt-4 grid max-w-xl gap-2" @submit.prevent="saveTransfer">
        <label for="transfer-owner">新负责人</label><select id="transfer-owner" v-model="targetOwner" required class="rounded border p-2"><option value="" disabled>请选择负责人</option><option v-for="target in targets" :key="target.userId" :value="target.userId">{{ target.name }} · {{ target.departmentName }}</option></select>
        <p v-if="!targets.length">暂无可转移的启用成员。</p><el-button native-type="submit" :disabled="saving || !targetOwner">确认转移</el-button>
      </form>
      <template v-if="customer.canTransfer"><h3 class="mt-6 font-semibold">最近操作审计（最多100条）</h3><p v-if="!audits.length">暂无审计记录。</p><ul><li v-for="item in audits" :key="item.id">{{ new Date(item.createdAt).toLocaleString() }} · {{ item.actorName }} · {{ auditLabel(item.action) }}<span v-if="item.action === 'customer.transfer'"> · 负责人 {{ item.fromOwnerId }} → {{ item.toOwnerId }}</span></li></ul></template>
      <h3 class="mb-3 mt-6 font-semibold">联系人</h3>
      <p v-if="contacts.some(item => item.masked)">只读权限下，电话和邮箱已脱敏。</p>
      <el-table :data="contacts" row-key="id" empty-text="暂无联系人">
        <el-table-column label="姓名" prop="name" />
        <el-table-column label="电话" prop="phone" />
        <el-table-column label="邮箱" prop="email" />
        <el-table-column label="职位" prop="title" />
        <el-table-column v-if="customer.canUpdate" label="操作">
          <template #default="{ row }">
            <el-button :disabled="saving || row.masked" @click="editContact(row)">编辑</el-button>
            <el-button :disabled="saving || row.masked" type="danger" @click="removeContact(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <form v-if="customer.canUpdate" aria-label="联系人表单" class="mt-4 grid max-w-xl gap-2" @submit.prevent="saveContact">
        <h4>{{ editingContact ? '编辑联系人' : '新增联系人' }}</h4>
        <label for="contact-name">联系人姓名</label><el-input id="contact-name" v-model="contactDraft.name" required maxlength="128" />
        <label for="contact-phone">联系人电话</label><el-input id="contact-phone" v-model="contactDraft.phone" maxlength="32" />
        <label for="contact-email">联系人邮箱</label><el-input id="contact-email" v-model="contactDraft.email" type="email" maxlength="254" />
        <label for="contact-title">联系人职位</label><el-input id="contact-title" v-model="contactDraft.title" maxlength="64" />
        <div class="flex gap-3">
          <el-button type="primary" native-type="submit" :loading="saving">保存联系人</el-button>
          <el-button v-if="editingContact" :disabled="saving" @click="resetContact">取消编辑</el-button>
        </div>
      </form>
      <el-button v-if="customer.canDelete" class="mt-6" type="danger" :disabled="saving || contacts.length > 0 || history.total > 0" @click="removeCustomer">删除客户</el-button>
      <p v-if="history.total">已有跟进历史，客户将保留，不能删除。</p>
      <p v-if="customer.canDelete && contacts.length">移除联系人后可删除客户。</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { createContact, deleteContact, deleteCustomer, getCustomer, listContacts, updateContact, updateCustomer, addFollowup, listFollowups, listTransferTargets, transferCustomer, listCustomerAudit, type FollowupPage, type TransferTarget, type AuditEvent, type Contact, type CustomerDetail } from '@/api/dealflow-crm'
import { useDealflowSession } from '@/pinia/modules/dealflow-session'
const props = defineProps<{ tenantId: string }>()
const route = useRoute(), router = useRouter(), session = useDealflowSession()
const id = String(route.params.id)
const customer = ref<CustomerDetail>()
const contacts = ref<Contact[]>([])
const history = ref<FollowupPage>({ list: [], total: 0, page: 1, pageSize: 20 })
const targets = ref<TransferTarget[]>([]), audits = ref<AuditEvent[]>([])
const targetOwner = ref('')
const followup = reactive({ content: '', next: '' })
const draft = reactive({ name: '', industry: '', remark: '' })
const contactDraft = reactive({ name: '', phone: '', email: '', title: '' })
const editingContact = ref('')
const loading = ref(true), saving = ref(false), error = ref('')
const request = new AbortController()
watch(() => session.menu, () => request.abort(), { flush: 'sync' })
onBeforeUnmount(() => request.abort())
onMounted(load)
async function load() {
  loading.value = true
  try {
    const [row, people, timeline] = await Promise.all([getCustomer(props.tenantId, id, request.signal), listContacts(props.tenantId, id, request.signal), listFollowups(props.tenantId, id, history.value.page, request.signal)])
    const [members, events] = row.canTransfer ? await Promise.all([listTransferTargets(props.tenantId, id, request.signal), listCustomerAudit(props.tenantId, id, request.signal)]) : [[], []]
    if (request.signal.aborted) return
    customer.value = row
    contacts.value = people
    history.value = timeline; targets.value = members; audits.value = events
    targetOwner.value = ''
    followup.next = row.nextFollowupAt ? localDate(row.nextFollowupAt) : ''
    Object.assign(draft, { name: row.name, industry: row.industry, remark: row.remark })
  } catch {
    if (!request.signal.aborted) { customer.value = undefined; contacts.value = []; error.value = '客户不存在、无权访问或暂时无法加载。' }
  } finally { if (!request.signal.aborted) loading.value = false }
}
async function mutate(action: () => Promise<unknown>) {
  if (saving.value || request.signal.aborted) return
  saving.value = true
  error.value = ''
  try {
    await action()
    if (!request.signal.aborted) await load()
  } catch { if (!request.signal.aborted) error.value = '无法保存，请检查表单和当前权限。' }
  finally { if (!request.signal.aborted) saving.value = false }
}
async function saveCustomer() {
  if (!customer.value?.canUpdate) return
  await mutate(() => updateCustomer(props.tenantId, id, { ...draft }, request.signal))
}
function resetContact() {
  editingContact.value = ''
  Object.assign(contactDraft, { name: '', phone: '', email: '', title: '' })
}
function editContact(row: Contact) {
  if (row.masked) return
  editingContact.value = row.id
  Object.assign(contactDraft, { name: row.name, phone: row.phone, email: row.email, title: row.title })
}
async function saveContact() {
  if (!customer.value?.canUpdate) return
  await mutate(async () => {
    if (editingContact.value) await updateContact(props.tenantId, editingContact.value, { ...contactDraft }, request.signal)
    else await createContact(props.tenantId, id, { ...contactDraft }, request.signal)
    if (!request.signal.aborted) resetContact()
  })
}
async function removeContact(contactId: string) {
  try { await ElMessageBox.confirm('确认删除这位联系人？', '删除联系人', { type: 'warning' }) } catch { return }
  if (!customer.value?.canUpdate) return
  await mutate(() => deleteContact(props.tenantId, contactId, request.signal))
}
async function removeCustomer() {
  try { await ElMessageBox.confirm('确认删除这个客户？', '删除客户', { type: 'warning' }) } catch { return }
  if (saving.value || request.signal.aborted || !customer.value?.canDelete) return
  saving.value = true
  try {
    await deleteCustomer(props.tenantId, id, request.signal)
    if (!request.signal.aborted) await router.replace({ name: 'DealflowCustomers' })
  } catch { if (!request.signal.aborted) error.value = '无法删除客户，请先移除联系人并检查当前权限。' }
  finally { if (!request.signal.aborted) saving.value = false }
}
function localDate(value: string) { const d = new Date(value); return new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16) }
function auditLabel(action: string) { return ({ 'customer.create': '创建客户', 'customer.update': '更新客户', 'customer.delete': '删除客户', 'customer.transfer': '转移负责人', 'contact.create': '新增联系人', 'contact.update': '更新联系人', 'contact.delete': '删除联系人', 'followup.create': '新增跟进' } as Record<string, string>)[action] || action }
async function historyPage(page: number) { history.value.page = page; await load() }
async function saveFollowup() {
  if (!customer.value?.canUpdate) return
  await mutate(async () => {
    await addFollowup(props.tenantId, id, { content: followup.content, nextFollowupAt: followup.next ? new Date(followup.next).toISOString() : null }, request.signal)
    followup.content = ''; history.value.page = 1
  })
}
async function saveTransfer() {
  if (!customer.value?.canTransfer || !targetOwner.value) return
  try { await ElMessageBox.confirm('确认转移负责人并同步部门？原负责人可能失去访问权限。', '转移负责人', { type: 'warning' }) } catch { return }
  await mutate(() => transferCustomer(props.tenantId, id, targetOwner.value, request.signal))
}
</script>
