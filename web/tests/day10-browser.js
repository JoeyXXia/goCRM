async (page) => {
  const fixture = /* DAY10_FIXTURE */ null
  const base = new URL('/', page.url()).href
  const writes = []
  page.on('request', request => {
    if (['POST', 'PUT'].includes(request.method()) && request.url().includes('/api/dealflow/')) {
      writes.push(request.postDataJSON())
    }
  })
  const check = (condition, message) => { if (!condition) throw new Error(message) }
  const confirm = async () => {
    await page.getByRole('dialog').getByRole('button', { name: /确定|Confirm|OK/ }).click()
  }
  try {
    await page.goto(base)
    await page.evaluate(token => { localStorage.setItem('token', token); sessionStorage.clear() }, fixture.token)
    await page.goto('about:blank')
    await page.goto(base + '#/dealflow')
    const picker = page.getByLabel('当前企业')
    await page.getByText('当前角色：销售').waitFor()
    await page.getByRole('link', { name: '客户', exact: true }).click()
    const form = page.getByRole('form', { name: '新增客户' })
    await form.getByLabel('客户名称', { exact: true }).fill('浏览器客户甲')
    await form.getByLabel('行业', { exact: true }).fill('制造')
    await form.getByLabel('备注', { exact: true }).fill('真实 API 浏览器验收')
    const created = page.waitForResponse(response => response.url().endsWith('/dealflow/customers') && response.request().method() === 'POST')
    await form.getByRole('button', { name: '创建客户' }).click()
    const customer = (await (await created).json()).data
    check(typeof customer.id === 'string' && customer.tenantId === String(fixture.tenantA), 'customer ID or tenant assignment differs')
    await page.getByRole('heading', { name: '浏览器客户甲', exact: true }).waitFor()
    const edit = page.getByRole('form', { name: '编辑客户' })
    await edit.getByLabel('客户名称', { exact: true }).fill('浏览器客户甲更新')
    await edit.getByRole('button', { name: '保存客户' }).click()
    await page.getByRole('heading', { name: '浏览器客户甲更新', exact: true }).waitFor()
    const contact = page.getByRole('form', { name: '联系人表单' })
    await contact.getByLabel('联系人姓名').fill('联系人甲')
    await contact.getByLabel('联系人电话').fill('12345')
    await contact.getByLabel('联系人邮箱').fill('purchase@example.test')
    await contact.getByLabel('联系人职位').fill('采购')
    await contact.getByRole('button', { name: '保存联系人' }).click()
    await page.getByRole('cell', { name: '联系人甲', exact: true }).waitFor()
    await page.getByRole('row').filter({ hasText: '联系人甲' }).getByRole('button', { name: '编辑', exact: true }).click()
    await contact.getByLabel('联系人电话').fill('67890')
    await contact.getByRole('button', { name: '保存联系人' }).click()
    await page.getByRole('cell', { name: '67890', exact: true }).waitFor()
    await page.screenshot({ path: 'output/day10-customer.png', fullPage: true })
    await picker.selectOption(String(fixture.tenantB))
    await page.getByText('当前角色：财务').waitFor()
    await page.getByRole('link', { name: '客户', exact: true }).click()
    await page.getByRole('link', { name: '企业 B 客户', exact: true }).click()
    await page.getByRole('cell', { name: 'B 联系人', exact: true }).waitFor()
    check(await page.getByRole('form', { name: '编辑客户' }).count() === 0, 'read-only enterprise showed customer editor')
    check(await page.getByRole('form', { name: '联系人表单' }).count() === 0, 'read-only enterprise showed contact editor')
    check(await page.getByRole('heading', { name: '浏览器客户甲更新' }).count() === 0, 'A detail remained visible in B')
    await page.goto(base + '#/dealflow/customers/' + customer.id)
    await page.getByText('客户不存在、无权访问或暂时无法加载。').waitFor()
    await page.getByText('确定', { exact: true }).click()
    await picker.selectOption(String(fixture.tenantA))
    await page.getByText('当前角色：销售').waitFor()
    await page.getByRole('link', { name: '客户', exact: true }).click()
    await page.locator('#customer-keyword').fill('浏览器客户甲更新')
    await page.getByLabel('排序', { exact: true }).selectOption('name')
    await page.getByLabel('方向', { exact: true }).selectOption('asc')
    await page.getByRole('button', { name: '查询', exact: true }).click()
    await page.getByRole('link', { name: '浏览器客户甲更新', exact: true }).click()
    await page.getByRole('cell', { name: '联系人甲', exact: true }).waitFor()
    await page.getByRole('row').filter({ hasText: '联系人甲' }).getByRole('button', { name: '删除', exact: true }).click()
    await confirm()
    await page.getByText('暂无联系人', { exact: true }).waitFor()
    await page.getByRole('button', { name: '删除客户', exact: true }).click()
    await confirm()
    await page.waitForURL(base + '#/dealflow/customers')
    await page.getByText('暂无可见客户', { exact: true }).waitFor()
    check(writes.length >= 4 && writes.every(body => !('ownerId' in body) && !('tenantId' in body) && !('departmentId' in body)), 'UI sent ownership fields')
    return 'Passed: real MySQL customer/contact create, edit, delete, search, read-only controls, cross-tenant direct URL and enterprise switch'
  } finally {
    await page.request.post('http://127.0.0.1:8888/__day10/stop?key=' + encodeURIComponent(fixture.stopKey))
  }
}
