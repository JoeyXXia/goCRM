async (page) => {
  const base = new URL('/', page.url()).href
  const a = { id: '9007199254740992', name: 'Tenant A', departmentId: '101', roleKey: 'tenant_admin' }
  const b = { id: '9007199254740993', name: 'Tenant B', departmentId: '202', roleKey: 'sales' }
  const response = (data) => ({ code: 0, data, msg: 'ok' })
  const requests = []
  let workspaces = [a, b]
  let releaseOldDetail
  let markOldDetailSettled
  let delayDetail = true
  let nameA = 'Department A'
  const oldDetail = new Promise(resolve => { releaseOldDetail = resolve })
  const oldDetailSettled = new Promise(resolve => { markOldDetailSettled = resolve })
  const check = (condition, message) => {
    if (!condition) throw new Error(message)
  }

  await page.context().unrouteAll()
  await page.context().route('**/api/**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    const tenantId = request.headers()['x-tenant-id']
    requests.push({ path, tenantId, method: request.method() })
    if (path === '/api/base/captcha') {
      return route.fulfill({ json: response({ captchaId: 'fixture', picPath: '' }) })
    }
    if (path === '/api/dealflow/workspaces') {
      check(!tenantId, 'workspace discovery sent a tenant selection')
      return route.fulfill({ json: response(workspaces) })
    }
    if (path === '/api/dealflow/menu') {
      check(tenantId === a.id || tenantId === b.id, 'menu lost tenant ID precision')
      return route.fulfill({ json: response([{ key: 'departments', canRename: tenantId === a.id }]) })
    }
    if (path === '/api/dealflow/departments/101' && tenantId === a.id) {
      if (request.method() === 'PATCH') {
        nameA = request.postDataJSON().name
        return route.fulfill({ json: response(null) })
      }
      const delayed = delayDetail
      if (delayed) await oldDetail
      try {
        await route.fulfill({ json: response({ name: nameA, code: 'a' }) })
      } catch (error) {
        if (!delayed || !/cancell?ed|abort|invalid interception/i.test(error.message)) throw error
      } finally {
        if (delayed) markOldDetailSettled()
      }
      return
    }
    if (path === '/api/dealflow/departments/202' && tenantId === b.id) {
      return route.fulfill({ json: response({ name: 'Department B', code: 'b' }) })
    }
    return route.fulfill({ status: 404, json: { code: 7, data: null, msg: 'not found' } })
  })

  await page.goto(base + '#/dealflow/departments/202/rename')
  await page.evaluate(() => { localStorage.removeItem('token'); sessionStorage.clear() })
  await page.reload()
  await page.waitForURL(url => url.hash.startsWith('#/login?redirect='))
  await page.evaluate(() => { localStorage.setItem('token', 'day6-browser-fixture') })
  await page.goto('about:blank')
  await page.goto(base + '#/dealflow')
  const picker = page.getByLabel('当前企业')
  const myDepartment = page.getByRole('link', { name: '我的部门' })
  await myDepartment.waitFor()
  check(await picker.inputValue() === a.id, 'default selection should be A')
  await page.getByText('当前角色：企业管理员').waitFor()
  const requestA = page.waitForRequest(request => request.url().endsWith('/departments/101'))
  const failedA = page.waitForEvent('requestfailed', request => request.url().endsWith('/departments/101'))
  await myDepartment.click()
  await requestA
  await page.getByText('正在加载部门…').waitFor()
  await picker.selectOption(b.id)
  const canceled = await failedA
  check(/abort|cancel/i.test(canceled.failure()?.errorText ?? ''), 'A detail request was not canceled')
  await page.waitForURL(base + '#/dealflow')
  await page.getByText('当前角色：销售').waitFor()
  await myDepartment.click()
  await page.getByRole('heading', { name: 'Department B' }).waitFor()
  releaseOldDetail()
  await oldDetailSettled
  check(await page.getByRole('heading', { name: 'Department A' }).count() === 0, 'late A detail appeared in B')
  check(await page.getByRole('link', { name: '修改部门名称' }).count() === 0, 'A rename permission remained visible')
  check(await page.getByRole('dialog').count() === 0, 'cancellation showed a global error dialog')

  const departmentRequests = requests.filter(item => item.path.includes('/departments/')).length
  await page.goto(base + '#/dealflow/departments/202/rename')
  await page.getByText('无权访问此页面。').waitFor()
  await page.reload()
  await page.getByText('无权访问此页面。').waitFor()
  check(await picker.inputValue() === b.id, 'refresh did not restore B')
  check(requests.filter(item => item.path.includes('/departments/')).length === departmentRequests, 'denied route requested department data')

  workspaces = [a]
  delayDetail = false
  await page.goto(base + '#/dealflow')
  await page.reload()
  await page.getByText('当前角色：企业管理员').waitFor()
  check(await picker.inputValue() === a.id, 'revoked B selection was restored')
  await myDepartment.click()
  await page.getByRole('link', { name: '修改部门名称' }).click()
  await page.getByLabel('部门名称').fill('Department A renamed')
  await page.getByRole('button', { name: '保存' }).click()
  await page.getByRole('heading', { name: 'Department A renamed' }).waitFor()
  check(requests.some(item => item.method === 'PATCH' && item.tenantId === a.id), 'rename used the wrong tenant')
  return 'Passed: login redirect, A request cancellation, B isolation, direct URL denial, refresh, revocation fallback and A rename'
}
